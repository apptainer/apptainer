package hack

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
)

func openThisFile() (*os.File, error) {
	_, file, _, ok := runtime.Caller(1)
	if ok {
		return os.Open(filepath.Base(file))
	}
	return nil, fmt.Errorf("unable to determine test filename")
}

func TestUnsetFileFinalizer(t *testing.T) {
	// open the file and reset the finalizer
	f, err := openThisFile()
	assert.NilError(t, err)

	fd := f.Fd()

	// if something has changed in the os.File struct, this test will fail
	// and the code in UnsetFileFinalizer will need to be updated
	err = UnsetFileFinalizer(f)
	assert.NilError(t, err)

	// force garbage collection to run finalizer
	runtime.GC()

	time.Sleep(500 * time.Millisecond)

	// check the file descriptor is still open
	_, err = unix.FcntlInt(fd, unix.F_GETFL, 0)
	assert.NilError(t, err)
	assert.NilError(t, f.Close())

	// open the file without resetting the finalizer
	f, err = openThisFile()
	assert.NilError(t, err)

	fd = f.Fd()
	runtime.GC()

	// check the file descriptor is closed
	for {
		_, err = unix.FcntlInt(fd, unix.F_GETFL, 0)
		if err == nil {
			continue
		}
		assert.Error(t, err, "bad file descriptor")
		break
	}
}
