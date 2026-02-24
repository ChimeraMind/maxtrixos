package filesystems

import "golang.org/x/sys/unix"

// Mount wraps unix.Mount and can be replaced in tests.
var Mount = unix.Mount

// Unmount wraps unix.Unmount and can be replaced in tests.
var Unmount = unix.Unmount
