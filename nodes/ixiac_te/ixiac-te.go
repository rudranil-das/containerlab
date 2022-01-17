// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ixiac_te

import (
	"context"
	"strings"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

func init() {
	nodes.Register(nodes.NodeKindIXIACTE, func() nodes.Node {
		return new(ixiacTE)
	})
}

type ixiacTE struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (l *ixiacTE) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	l.cfg = cfg
	for _, o := range opts {
		o(l)
	}

	defEnv := map[string]string{
		"OPT_LISTEN_PORT":  "5555",
		"ARG_IFACE_LIST":   "virtual@af_packet,veth1",
		"OPT_NO_HUGEPAGES": "Yes",
	}
	l.cfg.Env = utils.MergeStringMaps(defEnv, l.cfg.Env)

	var envSb strings.Builder
	envSb.WriteString("./entrypoint.sh")

	l.cfg.Cmd = envSb.String()

	return nil
}

func (l *ixiacTE) Config() *types.NodeConfig { return l.cfg }

func (*ixiacTE) PreDeploy(_, _, _ string) error { return nil }

func (l *ixiacTE) Deploy(ctx context.Context) error {
	_, err := l.runtime.CreateContainer(ctx, l.cfg)
	return err
}

func (l *ixiacTE) PostDeploy(_ context.Context, _ map[string]nodes.Node) error { return nil }

func (l *ixiacTE) GetImages() map[string]string {
	images := make(map[string]string)
	images[nodes.ImageKey] = l.cfg.Image
	return images
}

func (*ixiacTE) WithMgmtNet(*types.MgmtNet)               {}
func (l *ixiacTE) WithRuntime(r runtime.ContainerRuntime) { l.runtime = r }
func (l *ixiacTE) GetRuntime() runtime.ContainerRuntime   { return l.runtime }

func (l *ixiacTE) Delete(ctx context.Context) error {
	return l.runtime.DeleteContainer(ctx, l.Config().LongName)
}

func (*ixiacTE) SaveConfig(_ context.Context) error {
	return nil
}
