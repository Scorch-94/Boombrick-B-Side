package remote

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// The input devices BlueZ makes for headphones say they are on Bluetooth.
const busBluetooth = 0x05

// lookAgain is how often /dev/input is looked through for headphones newly
// connected.
const lookAgain = 3 * time.Second

// runKeys reads the media keys of every Bluetooth input device, as BlueZ
// makes for connected headphones when no player is registered with it, until
// ctx ends.
func runKeys(ctx context.Context, r *Remote) {
	var (
		mu   sync.Mutex
		open = map[string]*os.File{}
		wg   sync.WaitGroup
	)
	defer func() {
		mu.Lock()
		for _, f := range open {
			f.Close() // ends its reader
		}
		mu.Unlock()
		wg.Wait()
	}()
	tick := time.NewTicker(lookAgain)
	defer tick.Stop()
	for {
		paths, _ := filepath.Glob("/dev/input/event*")
		for _, path := range paths {
			mu.Lock()
			_, have := open[path]
			mu.Unlock()
			if have {
				continue
			}
			f, name, ok := openBluetooth(path)
			if !ok {
				continue
			}
			mu.Lock()
			open[path] = f
			mu.Unlock()
			r.log.Info("bluetooth remote: reading keys", "device", name, "path", path)
			wg.Go(func() {
				readKeys(f, r, name)
				mu.Lock()
				delete(open, path)
				mu.Unlock()
				f.Close()
			})
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// openBluetooth opens the input device at path if it is on Bluetooth,
// returning its name.
func openBluetooth(path string) (*os.File, string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", false
	}
	var id [4]uint16 // bustype, vendor, product, version
	if ioctl(f, eviocgid, unsafe.Pointer(&id[0])) != nil || id[0] != busBluetooth {
		f.Close()
		return nil, "", false
	}
	var name [256]byte
	n := 0
	if ioctl(f, eviocgname(len(name)), unsafe.Pointer(&name[0])) == nil {
		for n < len(name) && name[n] != 0 {
			n++
		}
	}
	return f, string(name[:n]), true
}

// The ioctls asking an input device its ID and name.
const eviocgid = 0x80084502

func eviocgname(size int) uintptr { return 0x80000000 | uintptr(size)<<16 | 'E'<<8 | 0x06 }

func ioctl(f *os.File, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

// readKeys reads f's key presses until it fails, as when the headphones
// disconnect or f is closed.
func readKeys(f *os.File, r *Remote, name string) {
	size := int(unsafe.Sizeof(syscall.Timeval{})) + 8
	buf := make([]byte, size*16)
	for {
		n, err := f.Read(buf)
		if err != nil {
			return
		}
		for i := 0; i+size <= n; i += size {
			if c, ok := keyCommand(buf[i : i+size]); ok {
				r.Do(c, name+" (key)")
			}
		}
	}
}
