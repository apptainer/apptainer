package hack

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
)

// UnsetFileFinalizer unsets the finalizer for the os.File object
func UnsetFileFinalizer(obj *os.File) error {
	v := reflect.ValueOf(obj).Elem().FieldByName("file")
	if !v.IsValid() {
		return fmt.Errorf("*os.File file field not found: code update is required")
	}
	runtime.SetFinalizer(*(**interface{})(v.Addr().UnsafePointer()), nil)
	return nil
}
