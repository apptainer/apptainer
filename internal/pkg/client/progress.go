// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2018-2023, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

package client

import (
	"context"
	"io"

	"github.com/apptainer/apptainer/pkg/sylog"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

var defaultOption = []mpb.BarOption{
	mpb.PrependDecorators(
		decor.Counters(decor.SizeB1024(0), "%.1f / %.1f"),
	),
	mpb.AppendDecorators(
		decor.Percentage(),
		decor.AverageSpeed(decor.SizeB1024(0), " % .1f "),
		decor.AverageETA(decor.ET_STYLE_GO),
	),
}

func initProgressBar(totalSize int64) (*mpb.Progress, *mpb.Bar) {
	p := mpb.New()

	if totalSize > 0 {
		return p, p.AddBar(totalSize,
			mpb.PrependDecorators(
				decor.Counters(decor.SizeB1024(0), "%.1f / %.1f"),
			),
			mpb.AppendDecorators(
				decor.Percentage(),
				decor.AverageSpeed(decor.SizeB1024(0), " % .1f "),
				decor.AverageETA(decor.ET_STYLE_GO),
			),
		)
	}
	return p, p.AddBar(totalSize,
		mpb.PrependDecorators(
			decor.Current(decor.SizeB1024(0), "%.1f / ???"),
		),
		mpb.AppendDecorators(
			decor.AverageSpeed(decor.SizeB1024(0), " % .1f "),
		),
	)
}

// See: https://ixday.github.io/post/golang-cancel-copy/
type readerFunc func(p []byte) (n int, err error)

func (rf readerFunc) Read(p []byte) (n int, err error) { return rf(p) }

// ProgressCallback is a function that provides progress information copying from a Reader to a Writer
type ProgressCallback func(int64, io.Reader, io.Writer) error

// ProgressBarCallback returns a progress bar callback unless e.g. --quiet or lower loglevel is set
func ProgressBarCallback(ctx context.Context) ProgressCallback {
	if sylog.GetLevel() <= -1 {
		// If we don't need a bar visible, we just copy data through the callback func
		return func(totalSize int64, r io.Reader, w io.Writer) error {
			_, err := CopyWithContext(ctx, w, r)
			return err
		}
	}

	return func(totalSize int64, r io.Reader, w io.Writer) error {
		p, bar := initProgressBar(totalSize) //nolint:contextcheck

		// create proxy reader
		bodyProgress := bar.ProxyReader(r)
		defer bodyProgress.Close()

		written, err := CopyWithContext(ctx, w, bodyProgress)
		if err != nil {
			bar.Abort(true)
			return err
		}

		// Must ensure bar is complete for a download with unknown size, or it will hang.
		if totalSize <= 0 {
			bar.SetTotal(written, true)
		}
		p.Wait()

		return nil
	}
}

func CopyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (written int64, err error) {
	// Copy will call the Reader and Writer interface multiple time, in order
	// to copy by chunk (avoiding loading the whole file in memory).
	// I insert the ability to cancel before read time as it is the earliest
	// possible in the call process.
	written, err = io.Copy(dst, readerFunc(func(p []byte) (int, error) {
		// golang non-blocking channel: https://gobyexample.com/non-blocking-channel-operations
		select {
		// if context has been canceled
		case <-ctx.Done():
			// stop process and propagate "context canceled" error
			return 0, ctx.Err()
		default:
			// otherwise just run default io.Reader implementation
			return src.Read(p)
		}
	}))
	return written, err
}

// DownloadProgressBar is a progress bar that implements the container-library-client ProgressBar interface.
type DownloadProgressBar struct {
	bar *mpb.Bar
	p   *mpb.Progress
}

func (dpb *DownloadProgressBar) Init(contentLength int64) {
	if sylog.GetLevel() <= -1 {
		// we don't need a bar visible
		return
	}
	dpb.p, dpb.bar = initProgressBar(contentLength)
}

func (dpb *DownloadProgressBar) ProxyReader(r io.Reader) io.ReadCloser {
	return dpb.bar.ProxyReader(r)
}

func (dpb *DownloadProgressBar) IncrBy(n int) {
	if dpb.bar == nil {
		return
	}
	dpb.bar.IncrBy(n)
}

func (dpb *DownloadProgressBar) Abort(drop bool) {
	if dpb.bar == nil {
		return
	}
	dpb.bar.Abort(drop)
}

func (dpb *DownloadProgressBar) Wait() {
	if dpb.bar == nil {
		return
	}
	dpb.p.Wait()
}

// UploadProgressBar is a progress bar that implements the scs-library-client UploadCallback interface.
type UploadProgressBar struct {
	progress *mpb.Progress
	bar      *mpb.Bar
	r        io.Reader
}

func (upb *UploadProgressBar) InitUpload(totalSize int64, r io.Reader) {
	if sylog.GetLevel() <= -1 {
		// we don't need a bar visible
		upb.r = r
		return
	}
	upb.progress, upb.bar = initProgressBar(totalSize)
	upb.r = upb.bar.ProxyReader(r)
}

func (upb *UploadProgressBar) GetReader() io.Reader {
	return upb.r
}

func (upb *UploadProgressBar) Terminate() {
	if upb.bar == nil {
		return
	}
	upb.bar.Abort(true)
}

func (upb *UploadProgressBar) Finish() {
	if upb.progress == nil {
		return
	}
	// wait for our bar to complete and flush
	upb.progress.Wait()
}
