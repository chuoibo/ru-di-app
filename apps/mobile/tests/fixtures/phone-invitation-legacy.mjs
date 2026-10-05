/** A refusal to rename somebody who owns their name, and nothing broader. */
export function laTuChoiDoiTen(error) {
    if (typeof error !== "object" || error === null)
        return false;
    const e = error;
    return e.status === 403 && e.code === "permission_denied";
}
export async function moiBangSo(buoc, so, ten) {
    const personId = await buoc.layId(so);
    let tenDaDat = true;
    try {
        await buoc.datTen(personId, ten);
    }
    catch (error) {
        if (!laTuChoiDoiTen(error))
            throw error;
        tenDaDat = false;
    }
    await buoc.moi(personId);
    return { tenDaDat };
}
