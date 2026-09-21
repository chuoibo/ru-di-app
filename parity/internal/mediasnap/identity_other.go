//go:build !linux

package mediasnap

import "io/fs"

// fillIdentity leaves known false where we cannot read a file's device, inode
// and change time, so the cache never reuses a hash and every snapshot reads
// every file, as it did before the cache existed.
func fillIdentity(fs.FileInfo, *identity) {}
