"""Answer upload-sanitizer questions with the real Pillow in the parity image.

Run inside the pinned parity API image (the Go side starts it):

    docker run --rm -i --network none --entrypoint python \
        mobile-parity-api:7bf58e3d -c "$(cat scripts/render_media_sanitize_oracle.py)"

Protocol: one JSON object per line on stdin, one answer per line on stdout,
in the same order. Byte fields travel as base64 and are never written to disk.
Nothing here is committed output: the Go tests keep seeds and digests only.

Requests ("op"):

    sanitize     run app.media.images.sanitize_image on "b64" (or on the
                 bytes "gen" builds with Pillow) and report the outcome
    encode_png   Image.frombytes("RGBA", (w, h), b64).save(PNG)
    encode_jpeg  Image.frombytes("RGB", (w, h), b64).save(JPEG, quality=88,
                 optimize=True), the sanitizer's encoder call
    decode       Image.open(b64) + load(): mode, size, info keys, pixels
    gen          only build the "gen" input and return it
    metrics      luma SSIM and per-channel mean absolute error between two
                 encoded images "b64" (Pillow's) and "b64_other" (Go's)

"want" lists extra byte fields to return: "output" (encoded bytes),
"pixels" (the pixels handed to the encoder), "input" (generated input).
"""

from __future__ import annotations

import base64
import hashlib
import io
import json
import random
import sys
import traceback
import warnings

sys.path.insert(0, "/srv")

from PIL import Image, ImageOps  # noqa: E402

from app.media import images  # noqa: E402


def b64(data: bytes) -> str:
    return base64.b64encode(data).decode("ascii")


def unb64(text: str) -> bytes:
    return base64.b64decode(text)


def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


# --------------------------------------------------------------------------
# Inputs Go cannot build itself. Every generator is a pure function of its
# parameters and seed, so a case is reproducible from the request line.


def gen_bytes(count: int, rng: random.Random, pattern: str, width: int, stride: int):
    """Sample bytes for a row-major buffer of `stride` bytes per pixel."""

    if pattern == "noise":
        return bytes(rng.getrandbits(8) for _ in range(count))
    if pattern == "flat":
        pixel = bytes(rng.getrandbits(8) for _ in range(stride))
        return (pixel * (count // max(stride, 1) + 1))[:count]
    out = bytearray(count)
    base = [rng.randrange(256) for _ in range(stride)]
    for i in range(count):
        pixel, b = divmod(i, stride)
        y, x = divmod(pixel, max(width, 1))
        value = base[b] + (x * (b + 3) + y * (5 - b)) // 2 + rng.randrange(-6, 7)
        out[i] = max(0, min(255, value))
    return bytes(out)


def gen_image(gen: dict) -> Image.Image:
    """A seeded image of any Pillow mode.

    "mode" is the Pillow mode; "P" and "PA" take "colors" (palette entries,
    default 256) and "palette_mode" ("RGB" or "RGBA"); "1" is thresholded
    from L with no dither.
    """

    rng = random.Random(int(gen.get("seed", 1)))
    width = int(gen.get("w", 17))
    height = int(gen.get("h", 11))
    pattern = gen.get("pattern", "smooth")
    mode = gen.get("mode", "RGB")
    if mode == "1":
        gray = Image.frombytes(
            "L", (width, height), gen_bytes(width * height, rng, pattern, width, 1)
        )
        return gray.convert("1", dither=Image.Dither.NONE)
    if mode in ("P", "PA"):
        colors = int(gen.get("colors", 256))
        palette_mode = gen.get("palette_mode", "RGB")
        indices = bytes(
            v % colors for v in gen_bytes(width * height, rng, pattern, width, 1)
        )
        image = Image.frombytes("P", (width, height), indices)
        entries = bytes(rng.getrandbits(8) for _ in range(colors * len(palette_mode)))
        image.putpalette(entries, palette_mode)
        if mode == "PA":
            alpha = Image.frombytes(
                "L", (width, height), gen_bytes(width * height, rng, pattern, width, 1)
            )
            image = image.convert("PA")
            image.putalpha(alpha)
        return image
    stride = len(Image.new(mode, (1, 1)).tobytes())
    data = gen_bytes(width * height * stride, rng, pattern, width, stride)
    return Image.frombytes(mode, (width, height), data)


def from_json(value):
    """JSON to Python: {"b64": s} is bytes, {"tuple": [...]} a tuple."""

    if isinstance(value, dict):
        if set(value) == {"b64"}:
            return unb64(value["b64"])
        if set(value) == {"tuple"}:
            return tuple(from_json(v) for v in value["tuple"])
        return {k: from_json(v) for k, v in value.items()}
    if isinstance(value, list):
        return [from_json(v) for v in value]
    return value


def build(gen: dict) -> bytes:
    """Build an input with Pillow.

    kind "pillow": gen_image(gen) saved as "format" with the keyword
    arguments in "save" (bytes and tuples through from_json). Optional:
    "exif" {tag: value} merged into an Image.Exif passed as exif=...,
    "exif_orientation" (shortcut for tag 274), "pnginfo" a list of
    {"type": "text"|"ztxt"|"itxt"|"chunk", "key", "value"} (for "chunk":
    "cid" and "data" {"b64"}), "convert" a mode to convert to before saving.
    """

    kind = gen["kind"]
    if kind != "pillow":
        raise ValueError(f"unknown generator kind {kind!r}")
    image = gen_image(gen)
    if "convert" in gen:
        image = image.convert(gen["convert"])
    params = from_json(gen.get("save", {}))
    tags = {int(k): from_json(v) for k, v in gen.get("exif", {}).items()}
    if "exif_orientation" in gen:
        tags[0x0112] = int(gen["exif_orientation"])
    if tags:
        exif = Image.Exif()
        for tag, value in tags.items():
            exif[tag] = value
        params["exif"] = exif.tobytes()
    if "pnginfo" in gen:
        from PIL import PngImagePlugin

        info = PngImagePlugin.PngInfo()
        for item in gen["pnginfo"]:
            kind_ = item["type"]
            if kind_ == "text":
                info.add_text(item["key"], item["value"])
            elif kind_ == "ztxt":
                info.add_text(item["key"], item["value"], zip=True)
            elif kind_ == "itxt":
                info.add_itxt(item["key"], item["value"], zip=bool(item.get("zip")))
            elif kind_ == "chunk":
                info.add(item["cid"].encode("ascii"), unb64(item["data"]["b64"]))
        params["pnginfo"] = info
    out = io.BytesIO()
    image.save(out, format=gen["format"], **params)
    return out.getvalue()


# --------------------------------------------------------------------------


def info_summary(image: Image.Image) -> dict:
    summary = {}
    for key in ("transparency", "exif", "xmp", "XML:com.adobe.xmp"):
        if key in image.info:
            value = image.info[key]
            if isinstance(value, bytes):
                summary[key] = {"bytes": b64(value)}
            elif isinstance(value, str):
                summary[key] = {"str": value}
            elif isinstance(value, tuple):
                summary[key] = {"tuple": list(value)}
            else:
                summary[key] = {"repr": repr(value)}
    try:
        summary["orientation"] = repr(image.getexif().get(0x0112))
    except Exception as exc:  # the sanitizer would refuse; report the type
        summary["orientation_error"] = type(exc).__name__
    return summary


def op_sanitize(request: dict, want: set[str]) -> dict:
    raw = (
        unb64(request["b64"])
        if request.get("b64") is not None
        else build(request["gen"])
    )
    answer: dict = {}
    if "input" in want or request.get("gen") is not None:
        answer["input_sha256"] = sha(raw)
    if "input" in want:
        answer["b64_input"] = b64(raw)
    if "pixels" in want:
        answer.update(pixels_before_encode(raw))
    try:
        result = images.sanitize_image(raw)
    except images.ImageRejected as exc:
        answer.update(result="rejected", code=exc.code, detail=exc.detail)
        return answer
    except Exception as exc:
        answer.update(result="error", type=type(exc).__name__, message=str(exc))
        return answer
    answer.update(
        result="ok",
        sha256=sha(result.data),
        size=len(result.data),
        content_type=result.content_type,
        width=result.width,
        height=result.height,
    )
    if "output" in want:
        answer["b64_output"] = b64(result.data)
    return answer


def pixels_before_encode(raw: bytes) -> dict:
    """Replay sanitize_image up to the encoder and report its pixels."""

    try:
        with warnings.catch_warnings():
            warnings.simplefilter("error", Image.DecompressionBombWarning)
            with Image.open(io.BytesIO(raw)) as source:
                opened_mode = source.mode
                source.load()
                transposed = ImageOps.exif_transpose(source)
                transposed.load()
                has_alpha = "A" in transposed.getbands() or (
                    transposed.mode == "P" and "transparency" in transposed.info
                )
                mode = "RGBA" if has_alpha else "RGB"
                pixels = transposed.convert(mode).tobytes()
                return {
                    "opened_mode": opened_mode,
                    "format": source.format,
                    "pixels_sha256": sha(pixels),
                    "b64_pixels": b64(pixels),
                    "pixels_mode": mode,
                    "pixels_w": transposed.width,
                    "pixels_h": transposed.height,
                }
    except Exception as exc:
        return {"pixels_error": type(exc).__name__}


def op_encode(request: dict, want: set[str], fmt: str) -> dict:
    mode = "RGBA" if fmt == "PNG" else "RGB"
    image = Image.frombytes(mode, (request["w"], request["h"]), unb64(request["b64"]))
    out = io.BytesIO()
    try:
        if fmt == "PNG":
            image.save(out, format="PNG")
        else:
            image.save(out, format="JPEG", quality=88, optimize=True)
    except Exception as exc:
        return {"result": "error", "type": type(exc).__name__, "message": str(exc)}
    data = out.getvalue()
    answer = {"result": "ok", "sha256": sha(data), "size": len(data)}
    if "output" in want:
        answer["b64_output"] = b64(data)
    return answer


def op_decode(request: dict, want: set[str]) -> dict:
    raw = (
        unb64(request["b64"])
        if request.get("b64") is not None
        else build(request["gen"])
    )
    answer: dict = {"input_sha256": sha(raw)}
    if "input" in want:
        answer["b64_input"] = b64(raw)
    try:
        with warnings.catch_warnings():
            warnings.simplefilter("error", Image.DecompressionBombWarning)
            with Image.open(io.BytesIO(raw)) as image:
                answer.update(
                    format=image.format,
                    mode=image.mode,
                    width=image.width,
                    height=image.height,
                )
                image.load()
                answer.update(
                    result="ok",
                    loaded_mode=image.mode,
                    loaded_width=image.width,
                    loaded_height=image.height,
                    info=info_summary(image),
                )
                pixels = image.tobytes()
                answer["pixels_sha256"] = sha(pixels)
                if "pixels" in want:
                    answer["b64_pixels"] = b64(pixels)
                if image.mode in ("P", "PA") and image.palette is not None:
                    answer["palette_mode"] = image.palette.mode
                    answer["b64_palette"] = b64(
                        bytes(image.getpalette(image.palette.mode) or [])
                    )
    except Exception as exc:
        answer.update(result="error", type=type(exc).__name__, message=str(exc))
    return answer


def op_plugin(request: dict, want: set[str]) -> dict:
    """Run one registered open plugin on "b64" the way _open_core would.

    "format" names the entry of Image.OPEN. Reports accept(prefix), the
    factory outcome ("ok", "next" for the exceptions _open_core catches,
    "raise"), the bomb check, and load().
    """

    import struct

    raw = unb64(request["b64"])
    Image.init()
    factory, accept = Image.OPEN[request["format"]]
    answer: dict = {"result": "ok"}
    prefix = raw[:16]
    if accept is None:
        answer["accept"] = None
    else:
        try:
            verdict = accept(prefix)
        except (SyntaxError, IndexError, TypeError, struct.error) as exc:
            # _open_core calls accept inside its try: this is "not accepted".
            verdict = False
            answer["accept_raised"] = type(exc).__name__
        answer["accept"] = (
            verdict if isinstance(verdict, (bool, str)) else bool(verdict)
        )
    with warnings.catch_warnings():
        warnings.simplefilter("error", Image.DecompressionBombWarning)
        try:
            image = factory(io.BytesIO(raw), "")
        except (SyntaxError, IndexError, TypeError, struct.error) as exc:
            answer.update(open="next", type=type(exc).__name__, message=str(exc))
            return answer
        except BaseException as exc:
            answer.update(open="raise", type=type(exc).__name__, message=str(exc))
            return answer
        answer.update(
            open="ok", mode=image.mode, width=image.width, height=image.height
        )
        try:
            Image._decompression_bomb_check(image.size)
        except BaseException as exc:
            answer.update(bomb=type(exc).__name__)
            return answer
        try:
            image.load()
        except BaseException as exc:
            answer.update(load="raise", type=type(exc).__name__, message=str(exc))
            return answer
        answer.update(
            load="ok",
            loaded_mode=image.mode,
            loaded_width=image.width,
            loaded_height=image.height,
            info=info_summary(image),
        )
        pixels = image.tobytes()
        answer["pixels_sha256"] = sha(pixels)
        if "pixels" in want:
            answer["b64_pixels"] = b64(pixels)
    return answer


def op_metrics(request: dict) -> dict:
    import numpy as np

    first = Image.open(io.BytesIO(unb64(request["b64"])))
    second = Image.open(io.BytesIO(unb64(request["b64_other"])))
    if first.size != second.size:
        return {"result": "error", "type": "SizeMismatch", "message": ""}
    mode = "RGBA" if "A" in first.getbands() else "RGB"
    a = np.asarray(first.convert(mode), dtype=np.float64)
    b = np.asarray(second.convert(mode), dtype=np.float64)
    mae = [float(np.mean(np.abs(a[..., c] - b[..., c]))) for c in range(a.shape[2])]
    luma_a = np.asarray(first.convert("L"), dtype=np.float64)
    luma_b = np.asarray(second.convert("L"), dtype=np.float64)
    return {"result": "ok", "mae": mae, "ssim": ssim(luma_a, luma_b)}


def ssim(x, y) -> float:
    """Mean SSIM over 8x8 windows with stride 4 (or the whole image)."""

    import numpy as np

    c1 = (0.01 * 255) ** 2
    c2 = (0.03 * 255) ** 2
    height, width = x.shape
    win = 8 if min(height, width) >= 8 else min(height, width)
    step = 4 if win == 8 else win
    scores = []
    for top in range(0, height - win + 1, step):
        for left in range(0, width - win + 1, step):
            wx = x[top : top + win, left : left + win]
            wy = y[top : top + win, left : left + win]
            mx, my = wx.mean(), wy.mean()
            vx, vy = wx.var(), wy.var()
            cov = ((wx - mx) * (wy - my)).mean()
            scores.append(
                ((2 * mx * my + c1) * (2 * cov + c2))
                / ((mx * mx + my * my + c1) * (vx + vy + c2))
            )
    return float(np.mean(scores))


def main() -> None:
    out = sys.stdout
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        request = json.loads(line)
        want = set(request.get("want", []))
        op = request.get("op", "sanitize")
        try:
            if op == "sanitize":
                answer = op_sanitize(request, want)
            elif op == "encode_png":
                answer = op_encode(request, want, "PNG")
            elif op == "encode_jpeg":
                answer = op_encode(request, want, "JPEG")
            elif op == "decode":
                answer = op_decode(request, want)
            elif op == "gen":
                raw = build(request["gen"])
                answer = {
                    "result": "ok",
                    "input_sha256": sha(raw),
                    "b64_input": b64(raw),
                }
            elif op == "metrics":
                answer = op_metrics(request)
            elif op == "plugin":
                answer = op_plugin(request, want)
            else:
                answer = {"result": "error", "type": "BadOp", "message": op}
        except Exception:
            answer = {
                "result": "error",
                "type": "OracleCrash",
                "message": traceback.format_exc(),
            }
        answer["id"] = request.get("id")
        out.write(json.dumps(answer) + "\n")
        out.flush()


if __name__ == "__main__":
    main()
