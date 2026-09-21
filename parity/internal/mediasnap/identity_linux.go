package mediasnap

import (
	"io/fs"
	"syscall"
)

// fillIdentity adds what Linux tells us beyond the portable fields: the device
// and inode numbers, the link count, and the inode's change time. ctime is the
// strongest of them for a cache: a write bumps it, and unlike mtime no
// userspace call can hold it still or set it back.
func fillIdentity(info fs.FileInfo, id *identity) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	id.dev, id.ino, id.nlink = uint64(st.Dev), uint64(st.Ino), uint64(st.Nlink)
	id.ctime = st.Ctim.Nano()
	id.known = true
}
