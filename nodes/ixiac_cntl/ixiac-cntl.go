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

	defEnv := map[string]string{
		"ACCEPT_EULA":          "Yes",
		"HTTP_PORT":            "443",
		"DISABLE_USAGE_REPORT": "No",
		"DEBUG":                "No",
	}
	l.cfg.Env = utils.MergeStringMaps(defEnv, l.cfg.Env)

	var envSb strings.Builder
	if l.cfg.Env["ACCEPT_EULA"] == "Yes" {
		envSb.WriteString(" --accept-eula")
	}
	if l.cfg.Env["HTTP_PORT"] != "443" {
		envSb.WriteString(" --http-port " + l.cfg.Env["HTTP_PORT"])
	}
	if l.cfg.Env["DISABLE_USAGE_REPORT"] == "Yes" {
		envSb.WriteString(" --disable-app-usage-reporter")
	}
	if l.cfg.Env["DEBUG"] == "Yes" {
		envSb.WriteString(" --debug")
	}

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
