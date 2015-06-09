/* Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/*
 * The apr_md5_encode() routine uses much code obtained from the FreeBSD 3.0
 * MD5 crypt() function, which is licenced as follows:
 * ----------------------------------------------------------------------------
 * "THE BEER-WARE LICENSE" (Revision 42):
 * <phk@login.dknet.dk> wrote this file.  As long as you retain this notice you
 * can do whatever you want with this stuff. If we meet some day, and you think
 * this stuff is worth it, you can buy me a beer in return.   Poul-Henning Kamp
 * ----------------------------------------------------------------------------
 */

package htpasswd

import "crypto/md5"

const itoa64 = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// wordOutputs is a slice of tuples to be combined into a single uint64 and passed to to64.
// Each tuple is a slice of chunks.
// Each chunk is a pair of an offset and a number of bits to shift.
//
// l = (final[ 0]<<16) | (final[ 6]<<8) | final[12]; to64(p, l, 4); p += 4;
// l = (final[ 1]<<16) | (final[ 7]<<8) | final[13]; to64(p, l, 4); p += 4;
// l = (final[ 2]<<16) | (final[ 8]<<8) | final[14]; to64(p, l, 4); p += 4;
// l = (final[ 3]<<16) | (final[ 9]<<8) | final[15]; to64(p, l, 4); p += 4;
// l = (final[ 4]<<16) | (final[10]<<8) | final[ 5]; to64(p, l, 4); p += 4;
// l =                    final[11]                ; to64(p, l, 2); p += 2;
var wordOutputs = [][][2]int{
	{{0, 16}, {6, 8}, {12, 0}},
	{{1, 16}, {7, 8}, {13, 0}},
	{{2, 16}, {8, 8}, {14, 0}},
	{{3, 16}, {9, 8}, {15, 0}},
	{{4, 16}, {10, 8}, {5, 0}},
	{{11, 0}},
}

var magic = []byte("$apr1$")

// From http://svn.apache.org/viewvc/apr/apr-util/branches/1.3.x/crypto/apr_md5.c
func aprMD5(password, salt []byte) []byte {
	// Time to make the doughnuts...
	ctx := md5.New()
	// The password first, since that is what is most unknown
	ctx.Write(password)
	// Then our magic string
	ctx.Write(magic)
	// Then the raw salt
	ctx.Write(salt)

	// Then just as many characters of the MD5(pw, salt, pw)
	ctx1 := md5.New()
	ctx1.Write(password)
	ctx1.Write(salt)
	ctx1.Write(password)
	final := ctx1.Sum(nil)
	for i := len(password); i > 0; i -= md5.Size {
		if i > md5.Size {
			ctx.Write(final)
		} else {
			ctx.Write(final[:i])
		}
	}

	// Then something really weird...
	for i := len(password); i != 0; i >>= 1 {
		if i&1 != 0 {
			ctx.Write([]byte{0})
		} else {
			ctx.Write([]byte{password[0]})
		}
	}

	// And now, just to make sure things don't run too fast..
	// On a 60 Mhz Pentium this takes 34 msec, so you would
	// need 30 seconds to build a 1000 entry dictionary...
	final = ctx.Sum(nil)
	for i := 0; i < 1000; i++ {
		ctx1 := md5.New()

		if i&1 != 0 {
			ctx1.Write(password)
		} else {
			ctx1.Write(final)
		}

		if i%3 != 0 {
			ctx1.Write(salt)
		}

		if i%7 != 0 {
			ctx1.Write(password)
		}

		if i&1 != 0 {
			ctx1.Write(final)
		} else {
			ctx1.Write(password)
		}

		final = ctx1.Sum(nil)
	}

	result := []byte{}
	result = append(result, magic...)
	result = append(result, salt...)
	result = append(result, '$')

	for _, word := range wordOutputs {
		l := uint64(0)
		for _, chunk := range word {
			index := chunk[0]
			offset := chunk[1]
			l |= (uint64(final[index]) << uint(offset))
		}
		result = append(result, to64(l, len(word)+1)...)
	}

	return result
}

func to64(v uint64, n int) []byte {
	r := make([]byte, n)
	for i := 0; i < n; i++ {
		r[i] = itoa64[v&0x3f]
		v >>= 6
	}
	return r
}
