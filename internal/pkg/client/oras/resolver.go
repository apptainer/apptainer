// Copyright (c) Contributors to the Apptainer project, established as
//   Apptainer a Series of LF Projects LLC.
//   For website terms of use, trademark policy, privacy policy and other
//   project policies see https://lfprojects.org/policies
// Copyright (c) 2020, Control Command Inc. All rights reserved.
// Copyright (c) 2020-2022, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// This file is to add progress bar support for oras protocol.
package oras

import (
	"context"
	"io"

	"github.com/apptainer/apptainer/internal/pkg/client"
	libClient "github.com/apptainer/container-library-client/client"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/remotes"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type resolver struct {
	remotes.Resolver
}

type pusher struct {
	remotes.Pusher
}

type fetcher struct {
	remotes.Fetcher
}

type contentWriter struct {
	content.Writer
	mwriter io.Writer
	pb      libClient.ProgressBar
}

type fetchWriter struct {
	*io.PipeWriter
	pb libClient.ProgressBar
}

func (r *resolver) Resolve(ctx context.Context, ref string) (name string, desc ocispec.Descriptor, err error) {
	return r.Resolver.Resolve(ctx, ref)
}

func (r *resolver) Fetcher(ctx context.Context, ref string) (remotes.Fetcher, error) {
	f, err := r.Resolver.Fetcher(ctx, ref)
	if err != nil {
		return &fetcher{}, err
	}

	return &fetcher{f}, nil
}

func (r *resolver) Pusher(ctx context.Context, ref string) (remotes.Pusher, error) {
	p, err := r.Resolver.Pusher(ctx, ref)
	if err != nil {
		return &pusher{}, err
	}

	return &pusher{p}, nil
}

func (p *pusher) Push(ctx context.Context, desc ocispec.Descriptor) (content.Writer, error) {
	writer, err := p.Pusher.Push(ctx, desc)
	if err != nil {
		return &contentWriter{}, err
	}

	in, out := io.Pipe()
	mwriter := io.MultiWriter(writer, out)
	pb := &client.DownloadProgressBar{}
	pb.Init(desc.Size)

	go func() {
		_, err := io.Copy(io.Discard, in)
		if err != nil {
			pb.Abort(true)
			in.CloseWithError(err)
		}
		pb.Wait()
		in.Close()
	}()

	return &contentWriter{
		Writer:  writer,
		mwriter: mwriter,
		pb:      pb,
	}, nil
}

func (f *fetcher) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	reader, err := f.Fetcher.Fetch(ctx, desc)
	if err != nil {
		return &io.PipeReader{}, err
	}

	pb := &client.DownloadProgressBar{}
	pb.Init(desc.Size)

	in, out := io.Pipe()
	writer := &fetchWriter{
		PipeWriter: out,
		pb:         pb,
	}

	go func() {
		_, err := io.Copy(writer, reader)
		if err != nil {
			pb.Abort(true)
			out.CloseWithError(err)
		}
		pb.Wait()
		out.Close()
	}()

	return in, nil
}

func (w *contentWriter) Write(p []byte) (n int, err error) {
	n, err = w.mwriter.Write(p)
	if err != nil {
		w.pb.Abort(true)
		return n, err
	}
	w.pb.IncrBy(n)
	return n, err
}

func (pw *fetchWriter) Write(p []byte) (n int, err error) {
	n, err = pw.PipeWriter.Write(p)
	if err != nil {
		pw.pb.Abort(true)
		return n, err
	}
	pw.pb.IncrBy(n)
	return n, err
}
