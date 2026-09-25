//go:build eval

// Command rudi-eval is the AI engine's one eval binary (design 06 §1). It
// runs turns through aiharness.Engine.Run -- the worker's own seam -- with a
// recording Sink, and holds every request and every event to the invariants
// of design 06 §6.1. It is built only with `-tags eval`: never into the core
// image (the Dockerfile builds ./cmd/core alone), never into routes.json, and
// cmd/core never imports internal/aieval (aigate and scripts/eval_kich_ban.sh
// hold that line).
//
// Slice 6b has one model mode, `kich-ban`: the scripted stub of aiharness/llm
// over hand-written scripts. In that mode the binary builds no genai client
// and reads no API key, whatever the environment holds (invariant 10): there
// is no code path here that could, which TestKhongDungClientGenai in aieval
// holds on the source and TestKichBanKhongMoKetNoi holds at run time. The
// other modes of design 06 (`phat-lai`, `ghi`, `that`) and `--chi-buoc hieu`
// come with slices 9 and 18 and are refused until then.
//
// Two ways to drive it:
//
//	rudi-eval --mo-hinh kich-ban --bo <corpus.json> [--kich-ban <dir>] [--lap n]
//
// runs a whole corpus: one JSON line per run on stdout, then one line
// {"tong_ket": ...}; exit 0 only when every run did what its role asks and
// both sentinel cases (canary red where predicted, identity green) are there.
//
//	rudi-eval --mo-hinh kich-ban [--kich-ban <dir>]  < requests.jsonl
//
// speaks the line protocol of design 06 §3.2 on stdin/stdout, for the runner:
// {"op":"hang"} and {"op":"chay","ca":{...},"lap":n}. `hieu` and `dem_hang`
// belong to slices 9 and 15.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"mobile/services/core/internal/aieval"
)

func main() {
	os.Exit(chay(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// Exit codes: 0 green, 1 a run or the verdict is red, 2 the invocation or
// the input is wrong.
const (
	raXanh = 0
	raDo   = 1
	raSai  = 2
)

const moHinhKichBan = "kich-ban"

// moHinhChuaCo is every mode design 06 names that this slice has not built.
var moHinhChuaCo = map[string]string{
	"phat-lai": "lát 18 (cassette)",
	"ghi":      "lát 18 (cassette)",
	"that":     "lát 9 (lời gọi thật, cần Lead duyệt số lượng)",
}

func chay(ctx context.Context, args []string, in io.Reader, out, errw io.Writer) int {
	fs := flag.NewFlagSet("rudi-eval", flag.ContinueOnError)
	fs.SetOutput(errw)
	moHinh := fs.String("mo-hinh", "", "mô hình: kich-ban (lát này chỉ có chế độ này)")
	bo := fs.String("bo", "", "file corpus; vắng thì đọc giao thức dòng JSON trên stdin")
	kichBan := fs.String("kich-ban", "", "thư mục kịch bản stub; mặc định là ../kich_ban cạnh thư mục của corpus")
	lap := fs.Int("lap", 1, "số lần chạy kịch bản đúng của mỗi ca")
	chiBuoc := fs.String("chi-buoc", "", "chỉ chạy một chặng (hieu): lát 9")
	if err := fs.Parse(args); err != nil {
		return raSai
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(errw, "rudi-eval: tham số thừa %q\n", fs.Args())
		return raSai
	}
	if lat, ok := moHinhChuaCo[*moHinh]; ok {
		fmt.Fprintf(errw, "rudi-eval: --mo-hinh %s chưa có ở lát 6b; thuộc %s\n", *moHinh, lat)
		return raSai
	}
	if *moHinh != moHinhKichBan {
		fmt.Fprintf(errw, "rudi-eval: --mo-hinh phải là %s (có %q)\n", moHinhKichBan, *moHinh)
		return raSai
	}
	if *chiBuoc != "" {
		fmt.Fprintln(errw, "rudi-eval: --chi-buoc thuộc lát 9 (bước Understand chưa có)")
		return raSai
	}
	if *lap < 1 {
		fmt.Fprintln(errw, "rudi-eval: --lap phải ≥ 1")
		return raSai
	}
	dir := *kichBan
	if dir == "" {
		if *bo == "" {
			fmt.Fprintln(errw, "rudi-eval: giao thức dòng cần --kich-ban")
			return raSai
		}
		dir = filepath.Join(filepath.Dir(*bo), "..", "kich_ban")
	}
	kbs, err := aieval.DocKichBan(dir)
	if err != nil {
		fmt.Fprintf(errw, "rudi-eval: %v\n", err)
		return raSai
	}
	if *bo == "" {
		return giaoThuc(ctx, kbs, in, out, errw)
	}
	b, sha, err := aieval.DocBo(*bo, kbs)
	if err != nil {
		fmt.Fprintf(errw, "rudi-eval: %v\n", err)
		return raSai
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	tk, err := aieval.ChayBo(ctx, b, sha, kbs, *lap, func(r aieval.KetQuaChay) error {
		if !r.Dat {
			fmt.Fprintf(errw, "ĐỎ %s [%s, %s]: %s\n", r.CaID, r.Vai, r.KichBan, r.LyDo)
			for _, t := range r.Truot {
				fmt.Fprintf(errw, "    %s: %s\n", t.Kiem, t.ChiTiet)
			}
		}
		return enc.Encode(r)
	})
	if err != nil {
		fmt.Fprintf(errw, "rudi-eval: %v\n", err)
		return raSai
	}
	if err := enc.Encode(map[string]aieval.TongKet{"tong_ket": tk}); err != nil {
		return raSai
	}
	fmt.Fprintf(errw, "bộ %s: %d ca, %d lượt chạy, %d đạt, %d không đạt; kịch bản sai %d/%d trượt đúng chỗ; canary %s; đồng nhất %s\n",
		tk.Bo, tk.SoCa, tk.SoLuot, tk.Dat, tk.KhongDat, tk.SaiDat, tk.SoSai, trangThai(tk.Canary, "đỏ đúng chỗ"), trangThai(tk.DongNhat, "xanh"))
	if !tk.Xanh {
		return raDo
	}
	return raXanh
}

func trangThai(c aieval.CanhGac, dat string) string {
	switch {
	case !c.CoMat:
		return "VẮNG"
	case !c.Dat:
		return fmt.Sprintf("SAI (trượt %v)", c.Truot)
	}
	return dat
}

// yeuCau is one line of the protocol.
type yeuCau struct {
	Op  string          `json:"op"`
	Ca  json.RawMessage `json:"ca,omitempty"`
	Lap int             `json:"lap,omitempty"`
}

// giaoThuc answers the line protocol. A line it cannot run is answered with
// {"loi": ...}, and a run that does not do what its role asks is reported as
// it is; either makes the exit code red at the end. A line it cannot read
// stops it.
func giaoThuc(ctx context.Context, kbs map[string]aieval.KichBan, in io.Reader, out, errw io.Writer) int {
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	rc := raXanh
	loi := func(op, msg string) {
		_ = enc.Encode(map[string]string{"op": op, "loi": msg})
		rc = raDo
	}
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var y yeuCau
		dec := json.NewDecoder(bytes.NewReader(sc.Bytes()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&y); err != nil {
			fmt.Fprintf(errw, "rudi-eval: dòng không đọc được: %v\n", err)
			return raSai
		}
		switch y.Op {
		case "hang":
			_ = enc.Encode(map[string]aieval.Hang{"hang": aieval.DocHang()})
		case "chay":
			c, err := aieval.GiaiMaCa(y.Ca, kbs)
			if err != nil {
				loi(y.Op, err.Error())
				continue
			}
			n := y.Lap
			if n < 1 {
				n = 1
			}
			for i := 1; i <= n; i++ {
				r, err := aieval.ChayDung(ctx, c, kbs, i)
				if err != nil {
					loi(y.Op, err.Error())
					break
				}
				_ = enc.Encode(r)
				if !r.Dat {
					rc = raDo
				}
			}
		case "hieu", "dem_hang":
			loi(y.Op, "chưa có ở lát 6b (hieu: lát 9; dem_hang: lát 15)")
		default:
			loi(y.Op, "op lạ")
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintf(errw, "rudi-eval: %v\n", err)
		return raSai
	}
	return rc
}
