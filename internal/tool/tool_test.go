// Copyright 2021-present The Atlas Authors. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package tool_test

import (
	"fmt"
	"io/fs"
	"testing"
	"time"

	"ariga.io/atlas/internal/tool"
	"ariga.io/atlas/sql/migrate"
	"github.com/stretchr/testify/require"
)

var plan = &migrate.Plan{
	Name:       "tooling-plan",
	Reversible: true,
	Changes: []*migrate.Change{
		{Cmd: "CREATE TABLE t1(c int)", Reverse: "DROP TABLE t1 IF EXISTS", Comment: "create table t1"},
		{Cmd: "CREATE TABLE t2(c int)", Reverse: "DROP TABLE t2", Comment: "create table t2"},
	},
}

func TestFormatters(t *testing.T) {
	v := time.Now().Format("20060102150405")
	for _, tt := range []struct {
		name     string
		fmt      func() (migrate.Formatter, error)
		expected map[string]string
	}{
		{
			"golang-migrate/migrate",
			tool.NewGolangMigrateFormatter,
			map[string]string{
				v + "_tooling-plan.up.sql": `-- create table t1
CREATE TABLE t1(c int);
-- create table t2
CREATE TABLE t2(c int);
`,
				v + "_tooling-plan.down.sql": `-- reverse: create table t2
DROP TABLE t2;
-- reverse: create table t1
DROP TABLE t1 IF EXISTS;
`,
			},
		},
		{
			"pressly/goose",
			tool.NewGooseFormatter,
			map[string]string{
				v + "_tooling-plan.sql": `-- +goose Up
-- create table t1
CREATE TABLE t1(c int);
-- create table t2
CREATE TABLE t2(c int);

-- +goose Down
-- reverse: create table t2
DROP TABLE t2;
-- reverse: create table t1
DROP TABLE t1 IF EXISTS;
`,
			},
		},
		{
			"flyway",
			tool.NewFlywayFormatter,
			map[string]string{
				"V" + v + "__tooling-plan.sql": `-- create table t1
CREATE TABLE t1(c int);
-- create table t2
CREATE TABLE t2(c int);
`,
				"U" + v + "__tooling-plan.sql": `-- reverse: create table t2
DROP TABLE t2;
-- reverse: create table t1
DROP TABLE t1 IF EXISTS;
`,
			},
		},
		{
			"liquibase",
			tool.NewLiquibaseFormatter,
			map[string]string{
				v + "_tooling-plan.sql": fmt.Sprintf(`--liquibase formatted sql
--changeset atlas:%s-1
--comment: create table t1
CREATE TABLE t1(c int);
--rollback: DROP TABLE t1 IF EXISTS;

--changeset atlas:%s-2
--comment: create table t2
CREATE TABLE t2(c int);
--rollback: DROP TABLE t2;
`, v, v),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := dir(t)
			fmt, err := tt.fmt()
			require.NoError(t, err)
			pl := migrate.NewPlanner(nil, d, migrate.WithFormatter(fmt), migrate.DisableChecksum())
			require.NotNil(t, pl)
			require.NoError(t, pl.WritePlan(plan))
			require.Equal(t, len(tt.expected), countFiles(t, d))
			for name, content := range tt.expected {
				requireFileEqual(t, d, name, content)
			}
		})
	}
}

func dir(t *testing.T) migrate.Dir {
	p := t.TempDir()
	d, err := migrate.NewLocalDir(p)
	require.NoError(t, err)
	return d
}

func countFiles(t *testing.T, d migrate.Dir) int {
	files, err := fs.ReadDir(d, "")
	require.NoError(t, err)
	return len(files)
}

func requireFileEqual(t *testing.T, d migrate.Dir, name, contents string) {
	c, err := fs.ReadFile(d, name)
	require.NoError(t, err)
	require.Equal(t, contents, string(c))
}