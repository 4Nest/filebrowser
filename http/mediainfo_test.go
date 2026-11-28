package fbhttp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractBDInfoQuickSummaryCRLF(t *testing.T) {
	sample := "DISC INFO:\r\n\r\nDisc Title: TEST\r\n\r\n" +
		"QUICK SUMMARY:\r\n\r\n\r\n" +
		"Disc Title: TEST\r\n" +
		"Disc Size: 24,358,546,475 bytes\r\n" +
		"Length: 1:48:50.523\r\n" +
		"Total Bitrate: 29.57 Mbps\r\n" +
		"Video: MPEG-4 AVC Video / 23000 kbps / 1080p\r\n" +
		"[/code]\r\n\r\nFILES:\r\n00001.M2TS\r\n"

	got := extractBDInfoQuickSummary(sample)
	require.Contains(t, got, "QUICK SUMMARY:")
	require.Contains(t, got, "Disc Size: 24,358,546,475 bytes")
	require.Contains(t, got, "Video: MPEG-4 AVC Video")
	require.NotContains(t, got, "FILES:")
	require.NotContains(t, got, "\r")
	require.NotContains(t, got, "\n\n\n")

	full := normalizeBDInfoText(sample)
	require.NotContains(t, full, "\r")
	require.NotContains(t, full, "\n\n\n")
}
