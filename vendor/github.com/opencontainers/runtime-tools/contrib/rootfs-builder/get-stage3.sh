#!/bin/sh
#
# Download the current Gentoo stage3
#
# Copyright (C) 2014-2018 W. Trevor King <wking@tremily.us>
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

die()
{
	echo "$1"
	exit 1
}

MIRROR="${MIRROR:-http://distfiles.gentoo.org/}"
if test -n "${STAGE3}"
then
	if test -n "${STAGE3_ARCH}"
	then
		die 'if you set STAGE3, you do not need to set STAGE3_ARCH'
	fi
	if test -n "${DATE}"
	then
		die 'if you set STAGE3, you do not need to set DATE'
	fi
	STAGE3_ARCH=$(echo "${STAGE3}" | sed -n 's/stage3-\([^-]*\)-.*/\1/p')
	if test -z "${STAGE3_ARCH}"
	then
		die "could not calculate STAGE3_ARCH from ${STAGE3}"
	fi
	DATE=$(echo "${STAGE3}" | sed -n "s/stage3-${STAGE3_ARCH}-\([0-9TZ]*\)[.]tar[.].*/\1/p")
	if test -z "${DATE}"
	then
		die "could not calculate DATE from ${STAGE3}"
	fi
else
	STAGE3_ARCH="${STAGE3_ARCH:-amd64}"
fi

if test -z "${BASE_ARCH}"
then
	case "${STAGE3_ARCH}" in
		arm*)
			BASE_ARCH=arm
			;;
		i[46]86)
			BASE_ARCH=x86
			;;
		ppc*)
			BASE_ARCH=ppc
			;;
		*)
			BASE_ARCH="${STAGE3_ARCH}"
			;;
	esac
fi

BASE_ARCH_URL="${BASE_ARCH_URL:-${MIRROR}releases/${BASE_ARCH}/autobuilds/}"

if test -z "${STAGE3}"
then
	LATEST=$(wget -O - "${BASE_ARCH_URL}latest-stage3.txt")
	if test -z "${DATE}"
	then
		DATE=$(echo "${LATEST}" | sed -n "s|/stage3-${STAGE3_ARCH}-[0-9TZ]*[.]tar.*||p")
		if test -z "${DATE}"
		then
			die "could not calculate DATE from ${BASE_ARCH_URL}latest-stage3.txt"
		fi
	fi

	STAGE3=$(echo "${LATEST}" | sed -n "s|${DATE}/\(stage3-${STAGE3_ARCH}-${DATE}[.]tar[.][^ ]*\) .*|\1|p")
	if test -z "${STAGE3}"
	then
		die "could not calculate STAGE3 from ${BASE_ARCH_URL}latest-stage3.txt"
	fi
fi

ARCH_URL="${ARCH_URL:-${BASE_ARCH_URL}${DATE}/}"
STAGE3_CONTENTS="${STAGE3_CONTENTS:-${STAGE3}.CONTENTS}"
STAGE3_DIGESTS="${STAGE3_DIGESTS:-${STAGE3}.DIGESTS.asc}"

COMPRESSION=$(echo "${STAGE3}" | sed -n 's/^.*[.]\([^.]*\)$/\1/p')
if test -z "${COMPRESSION}"
then
	die "could not calculate COMPRESSION from ${STAGE3}"
fi
for FILE in "${STAGE3}" "${STAGE3_CONTENTS}" "${STAGE3_DIGESTS}"; do
	if [ ! -f "downloads/${FILE}" ]; then
		wget -O "downloads/${FILE}" "${ARCH_URL}${FILE}"
		if [ "$?" -ne 0 ]; then
			rm -f "downloads/${FILE}" &&
			die "failed to download ${ARCH_URL}${FILE}"
		fi
	fi

	FILE_NOCOMPRESSION=$(echo "${FILE}" | sed "s/[.]${COMPRESSION}//")
	if [ "${FILE_NOCOMPRESSION}" = "${FILE}" ]; then
		die "unable to remove .${COMPRESSION} from ${FILE}"
	fi
	CURRENT=$(echo "${FILE_NOCOMPRESSION}" | sed "s/${DATE}/current/")
	(
		cd downloads &&
		rm -f "${CURRENT}" &&
		ln -s "${FILE}" "${CURRENT}" ||
		die "failed to link ${CURRENT} -> ${FILE}"
	)
done
