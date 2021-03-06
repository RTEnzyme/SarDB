// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package executor_test

import (
	"bytes"
	"context"
	"fmt"
	"runtime/pprof"
	"strings"

	. "github.com/pingcap/check"
	"github.com/pingcap/tidb/domain"
	"github.com/pingcap/tidb/parser/model"
	"github.com/pingcap/tidb/store/tikv"
	"github.com/pingcap/tidb/tablecodec"
	"github.com/pingcap/tidb/util/testkit"
)

func checkGoroutineExists(keyword string) bool {
	buf := new(bytes.Buffer)
	profile := pprof.Lookup("goroutine")
	profile.WriteTo(buf, 1)
	str := buf.String()
	return strings.Contains(str, keyword)
}

func (s *testSuite3) TestCopClientSend(c *C) {
	c.Skip("not stable")
	if _, ok := s.store.GetClient().(*tikv.CopClient); !ok {
		// Make sure the store is tikv store.
		return
	}
	tk := testkit.NewTestKit(c, s.store)
	tk.MustExec("use test")
	tk.MustExec("create table copclient (id int primary key)")

	// Insert 1000 rows.
	var values []string
	for i := 0; i < 1000; i++ {
		values = append(values, fmt.Sprintf("(%d)", i))
	}
	tk.MustExec("insert copclient values " + strings.Join(values, ","))

	// Get table ID for split.
	dom := domain.GetDomain(tk.Se)
	is := dom.InfoSchema()
	tbl, err := is.TableByName(model.NewCIStr("test"), model.NewCIStr("copclient"))
	c.Assert(err, IsNil)
	tblID := tbl.Meta().ID

	// Split the table.
	s.cluster.SplitTable(s.mvccStore, tblID, 100)

	ctx := context.Background()
	// Send coprocessor request when the table split.
	rs, err := tk.Exec("select sum(id) from copclient")
	c.Assert(err, IsNil)
	req := rs.NewChunk()
	err = rs.Next(ctx, req)
	c.Assert(err, IsNil)
	c.Assert(req.GetRow(0).GetInt64(0), Equals, int64(499500))
	rs.Close()

	// Split one region.
	key := tablecodec.EncodeRowKeyWithHandle(tblID, 500)
	region, _ := s.cluster.GetRegionByKey([]byte(key))
	peerID := s.cluster.AllocID()
	s.cluster.Split(region.GetId(), s.cluster.AllocID(), key, []uint64{peerID}, peerID)

	// Check again.
	rs, err = tk.Exec("select sum(id) from copclient")
	c.Assert(err, IsNil)
	req = rs.NewChunk()
	err = rs.Next(ctx, req)
	c.Assert(err, IsNil)
	c.Assert(req.GetRow(0).GetInt64(0), Equals, int64(499500))
	rs.Close()

	// Check there is no goroutine leak.
	rs, err = tk.Exec("select * from copclient order by id")
	c.Assert(err, IsNil)
	req = rs.NewChunk()
	err = rs.Next(ctx, req)
	c.Assert(err, IsNil)
	rs.Close()
	keyword := "(*copIterator).work"
	c.Check(checkGoroutineExists(keyword), IsFalse)
}

func (s *testSuite3) TestBigIntPK(c *C) {
	tk := testkit.NewTestKit(c, s.store)
	tk.MustExec("use test")
	tk.MustExec("create table t(a bigint unsigned primary key, b int, c int, index idx(a, b))")
	tk.MustExec("insert into t values(1, 1, 1), (9223372036854775807, 2, 2)")
	tk.MustQuery("select * from t use index(idx) order by a").Check(testkit.Rows("1 1 1", "9223372036854775807 2 2"))
}

func (s *testSuite3) TestUniqueKeyNullValueSelect(c *C) {
	tk := testkit.NewTestKit(c, s.store)
	tk.MustExec("use test")
	tk.MustExec("drop table if exists t")
	// test null in unique-key
	tk.MustExec("create table t (id int default null, c varchar(20), unique id (id));")
	tk.MustExec("insert t (c) values ('a'), ('b'), ('c');")
	res := tk.MustQuery("select * from t where id is null;")
	res.Check(testkit.Rows("<nil> a", "<nil> b", "<nil> c"))

	// test null in mul unique-key
	tk.MustExec("drop table t")
	tk.MustExec("create table t (id int default null, b int default 1, c varchar(20), unique id_c(id, b));")
	tk.MustExec("insert t (c) values ('a'), ('b'), ('c');")
	res = tk.MustQuery("select * from t where id is null and b = 1;")
	res.Check(testkit.Rows("<nil> 1 a", "<nil> 1 b", "<nil> 1 c"))

	tk.MustExec("drop table t")
	// test null in non-unique-key
	tk.MustExec("create table t (id int default null, c varchar(20), key id (id));")
	tk.MustExec("insert t (c) values ('a'), ('b'), ('c');")
	res = tk.MustQuery("select * from t where id is null;")
	res.Check(testkit.Rows("<nil> a", "<nil> b", "<nil> c"))
}

// TestIssue10178 contains tests for https://github.com/pingcap/tidb/issues/10178 .
func (s *testSuite3) TestIssue10178(c *C) {
	tk := testkit.NewTestKit(c, s.store)
	tk.MustExec("use test")
	tk.MustExec("drop table if exists t")
	tk.MustExec("create table t(a bigint unsigned primary key)")
	tk.MustExec("insert into t values(9223372036854775807), (18446744073709551615)")
	tk.MustQuery("select max(a) from t").Check(testkit.Rows("18446744073709551615"))
	tk.MustQuery("select * from t where a > 9223372036854775807").Check(testkit.Rows("18446744073709551615"))
	tk.MustQuery("select * from t where a < 9223372036854775808").Check(testkit.Rows("9223372036854775807"))
}

func (s *testSuite3) TestPushLimitDownIndexLookUpReader(c *C) {
	tk := testkit.NewTestKit(c, s.store)
	tk.MustExec("use test")
	tk.MustExec("drop table if exists tbl")
	tk.MustExec("create table tbl(a int, b int, c int, key idx_b_c(b,c))")
	tk.MustExec("insert into tbl values(1,1,1),(2,2,2),(3,3,3),(4,4,4),(5,5,5)")
	tk.MustQuery("select * from tbl use index(idx_b_c) where b > 1 limit 2,1").Check(testkit.Rows("4 4 4"))
	tk.MustQuery("select * from tbl use index(idx_b_c) where b > 4 limit 2,1").Check(testkit.Rows())
	tk.MustQuery("select * from tbl use index(idx_b_c) where b > 3 limit 2,1").Check(testkit.Rows())
	tk.MustQuery("select * from tbl use index(idx_b_c) where b > 2 limit 2,1").Check(testkit.Rows("5 5 5"))
	tk.MustQuery("select * from tbl use index(idx_b_c) where b > 1 limit 1").Check(testkit.Rows("2 2 2"))
	tk.MustQuery("select * from tbl use index(idx_b_c) where b > 1 order by b desc limit 2,1").Check(testkit.Rows("3 3 3"))
	tk.MustQuery("select * from tbl use index(idx_b_c) where b > 1 and c > 1 limit 2,1").Check(testkit.Rows("4 4 4"))
}

func (s *testSuite3) TestTFIDFOrder(c *C) {
	tk := testkit.NewTestKit(c, s.store)
	tk.MustExec("use test")
	tk.MustExec("drop table if exists tbl")
	tk.MustExec("create table tbl(a int, c varchar(255))")

}
