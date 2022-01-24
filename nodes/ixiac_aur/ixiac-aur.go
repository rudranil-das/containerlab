// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ixiac_cntl

import (
	"context"
	"strings"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

func init() {
	nodes.Register(nodes.NodeKindIXIACAUR, func() nodes.Node {
		return new(ixiacAur)
	})
}

type ixiacAur struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (l *ixiacAur) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	l.cfg = cfg
	for _, o := range opts {
		o(l)
	}

	defEnv := map[string]string{
		"PORT": "5600",
	}
	l.cfg.Env = utils.MergeStringMaps(defEnv, l.cfg.Env)

	var envSb strings.Builder
	if l.cfg.Env["PORT"] != "5600" {
		envSb.WriteString(" -port " + l.cfg.Env["PORT"])
	}

	l.cfg.Cmd = envSb.String()

	return nil
}

func (l *ixiacAur) Config() *types.NodeConfig { return l.cfg }

func (*ixiacAur) PreDeploy(_, _, _ string) error { return nil }

func (l *ixiacAur) Deploy(ctx context.Context) error {
	_, err := l.runtime.CreateContainer(ctx, l.cfg)
	return err
}

func (l *ixiacAur) PostDeploy(_ context.Context, _ map[string]nodes.Node) error { return nil }

func (l *ixiacAur) GetImages() map[string]string {
	images := make(map[string]string)
	images[nodes.ImageKey] = l.cfg.Image
	return images
}

func (*ixiacAur) WithMgmtNet(*types.MgmtNet)               {}
func (l *ixiacAur) WithRuntime(r runtime.ContainerRuntime) { l.runtime = r }
func (l *ixiacAur) GetRuntime() runtime.ContainerRuntime   { return l.runtime }

func (l *ixiacAur) Delete(ctx context.Context) error {
	return l.runtime.DeleteContainer(ctx, l.Config().LongName)
}

func (*ixiacAur) SaveConfig(_ context.Context) error {
	return nil
}
