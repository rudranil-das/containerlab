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
)

func init() {
	nodes.Register(nodes.NodeKindIXIACCntl, func() nodes.Node {
		return new(ixiacCntl)
	})
}

type ixiacCntl struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (l *ixiacCntl) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	l.cfg = cfg
	for _, o := range opts {
		o(l)
	}

	var envSb strings.Builder
	envSb.WriteString("--accept-eula ")
	l.cfg.Cmd = envSb.String()

	return nil
}

func (l *ixiacCntl) Config() *types.NodeConfig { return l.cfg }

func (*ixiacCntl) PreDeploy(_, _, _ string) error { return nil }

func (l *ixiacCntl) Deploy(ctx context.Context) error {
	_, err := l.runtime.CreateContainer(ctx, l.cfg)
	return err
}

func (l *ixiacCntl) PostDeploy(_ context.Context, _ map[string]nodes.Node) error { return nil }

func (l *ixiacCntl) GetImages() map[string]string {
	images := make(map[string]string)
	images[nodes.ImageKey] = l.cfg.Image
	return images
}

func (*ixiacCntl) WithMgmtNet(*types.MgmtNet)               {}
func (l *ixiacCntl) WithRuntime(r runtime.ContainerRuntime) { l.runtime = r }
func (l *ixiacCntl) GetRuntime() runtime.ContainerRuntime   { return l.runtime }

func (l *ixiacCntl) Delete(ctx context.Context) error {
	return l.runtime.DeleteContainer(ctx, l.Config().LongName)
}

func (*ixiacCntl) SaveConfig(_ context.Context) error {
	return nil
}
