// +build cgo,linux

package util

import (
	"errors"
	"unsafe"
)

/*
#cgo LDFLAGS: -lcrypt
#define _GNU_SOURCE
#include <crypt.h>
#include <errno.h>
#include <stdlib.h>
#include <string.h>

char *do_gnu_source_crypt_r(const char *key, const char *salt) {
	struct crypt_data data = { 0, };
	char *result = crypt_r(key, salt, &data);
	if (!result)
		return NULL;
	result = strdup(result);
	if (!result)
		errno = ENOMEM;
	return result;
}
*/
import "C"

func Crypt(key string, salt string) (string, error) {
	ckey := C.CString(key)
	csalt := C.CString(salt)
	defer C.free(unsafe.Pointer(ckey))
	defer C.free(unsafe.Pointer(csalt))

	cres, err := C.do_gnu_source_crypt_r(ckey, csalt)
	if cres == nil {
		if err == nil {
			err = errors.New("crypt() returned invalid result")
		}
		return "", err
	}

	defer C.free(unsafe.Pointer(cres))
	return C.GoString(cres), nil
}
