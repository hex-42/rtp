package codecs

// from https://github.com/ireader/media-server/blob/master/librtp/payload/rtp-h264-bitstream.c

func h264_startcode(data []byte, bytes int) int {
	for i := 2; i+1 < bytes; i++ {
		if data[i] == 0x01 && data[i-1] == 0x00 && data[i-2] == 0x00 {
			return i + 1
		}
	}
	return -1
}

/// @return >0-ok, <=0-error
func h264_avcc_length(h264 []byte, bytes int, avcc int) int {
	n := 0
	if !(3 <= avcc && avcc <= 4) {
		return -2
	}
	for i := 0; i < avcc && i < bytes; i++ {
		n = (n << 8) | int(h264[i])
	}
	if avcc >= bytes {
		return -1
	}
	return n
}

/// @return 1-true, 0-false
func h264_avcc_bitstream_valid(h264 []byte, bytes int, avcc int) bool {
	index := 0
	for avcc+1 < bytes {
		n := h264_avcc_length(h264[index:], bytes, avcc)
		if n < 0 || n+avcc > bytes {
			return false // invalid
		}

		index += n + avcc
		bytes -= n + avcc
	}
	return bytes == 0
}

/// @return 0-annexb, >0-avcc, <0-error
func h264_bitstream_format(h264 []byte, bytes int) int {
	if bytes < 4 {
		return -1
	}

	n := uint32(h264[0])<<16 | uint32(h264[1])<<8 | uint32(h264[2])
	if n == 0 && h264[3] <= 1 {
		return 0 // annexb
	} else if n == 1 {
		// try avcc & annexb
		if h264_avcc_bitstream_valid(h264, bytes, 4) {
			return 4
		}
		return 0
	}
	// try avcc 4/3 bytes
	if h264_avcc_bitstream_valid(h264, bytes, 4) {
		return 4
	} else if h264_avcc_bitstream_valid(h264, bytes, 3) {
		return 3
	}
	return -1
}

type AvccHandler func(nalu []byte, bytes int, last bool) int

func h264_avcc_nalu(h264 []byte, bytes int, avcc int, handler AvccHandler) int {
	ret := 0
	index := 0
	end := bytes
	n := h264_avcc_length(h264[index:], end-index, avcc)
	for ret == 0 && index+n+avcc <= end {
		if n > 0 {
			ret = handler(h264[index+avcc:], n, index+avcc+n >= end)
		} else {
			return n
		}

		index += n + avcc

		n = h264_avcc_length(h264[index:], end-index, avcc)
	}

	return ret
}

///@param[in] h264 H.264 byte stream format data(A set of NAL units)
func rtp_h264_annexb_nalu(h264 []byte, bytes int, handler AvccHandler) int {
	avcc := h264_bitstream_format(h264, bytes)
	if avcc > 0 {
		return h264_avcc_nalu(h264, bytes, avcc, handler)
	}

	end := bytes
	index := h264_startcode(h264, bytes)
	ret := 0
	for index >= 0 && ret == 0 {
		newIndex := h264_startcode(h264[index:], (int)(end-index))
		n := end - index
		if newIndex >= 0 {
			n = newIndex - 3
		}

		for n > 0 && h264[index+n-1] == 0 {
			n-- // filter tailing zero
		}

		if n > 0 {
			ret = handler(h264[index:], n, newIndex <= 0)
		}
		if newIndex <= 0 {
			break
		}

		index += newIndex
	}

	return ret
}
