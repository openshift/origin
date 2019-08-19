package transactional

import (
	. "gopkg.in/check.v1"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

var _ = Suite(&ConfigSuite{})

type ConfigSuite struct{}

func (s *ConfigSuite) TestSetConfigBase(c *C) {
	cfg := config.NewConfig()
	cfg.Core.Worktree = "foo"

	base := memory.NewStorage()
	err := base.SetConfig(cfg)
	c.Assert(err, IsNil)

	temporal := memory.NewStorage()
	cs := NewConfigStorage(base, temporal)

	cfg, err = cs.Config()
	c.Assert(err, IsNil)
	c.Assert(cfg.Core.Worktree, Equals, "foo")
}

func (s *ConfigSuite) TestSetConfigTemporal(c *C) {
	cfg := config.NewConfig()
	cfg.Core.Worktree = "foo"

	base := memory.NewStorage()
	err := base.SetConfig(cfg)
	c.Assert(err, IsNil)

	temporal := memory.NewStorage()

	cfg = config.NewConfig()
	cfg.Core.Worktree = "bar"

	cs := NewConfigStorage(base, temporal)
	err = cs.SetConfig(cfg)
	c.Assert(err, IsNil)

	baseCfg, err := base.Config()
	c.Assert(err, IsNil)
	c.Assert(baseCfg.Core.Worktree, Equals, "foo")

	temporalCfg, err := temporal.Config()
	c.Assert(err, IsNil)
	c.Assert(temporalCfg.Core.Worktree, Equals, "bar")

	cfg, err = cs.Config()
	c.Assert(err, IsNil)
	c.Assert(temporalCfg.Core.Worktree, Equals, "bar")
}

func (s *ConfigSuite) TestCommit(c *C) {
	cfg := config.NewConfig()
	cfg.Core.Worktree = "foo"

	base := memory.NewStorage()
	err := base.SetConfig(cfg)
	c.Assert(err, IsNil)

	temporal := memory.NewStorage()

	cfg = config.NewConfig()
	cfg.Core.Worktree = "bar"

	cs := NewConfigStorage(base, temporal)
	err = cs.SetConfig(cfg)
	c.Assert(err, IsNil)

	err = cs.Commit()
	c.Assert(err, IsNil)

	baseCfg, err := base.Config()
	c.Assert(err, IsNil)
	c.Assert(baseCfg.Core.Worktree, Equals, "bar")
}
