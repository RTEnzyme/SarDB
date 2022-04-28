// Copyright 2015 PingCAP, Inc.
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

package expression

import (
	"fmt"
	. "github.com/pingcap/check"
	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/parser/ast"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tidb/util/stringutil"
)

func (s *testEvaluatorSuite) TestLengthAndOctetLength(c *C) {
	cases := []struct {
		args     interface{}
		expected int64
		isNil    bool
		getErr   bool
	}{
		{"abc", 3, false, false},
		{"你好", 6, false, false},
		{1, 1, false, false},
		{3.14, 4, false, false},
		{nil, 0, true, false},
		{errors.New("must error"), 0, false, true},
	}

	lengthMethods := []string{ast.Length, ast.OctetLength}
	for _, lengthMethod := range lengthMethods {
		for _, t := range cases {
			f, err := newFunctionForTest(s.ctx, lengthMethod, s.primitiveValsToConstants([]interface{}{t.args})...)
			c.Assert(err, IsNil)
			d, err := f.Eval(chunk.Row{})
			if t.getErr {
				c.Assert(err, NotNil)
			} else {
				c.Assert(err, IsNil)
				if t.isNil {
					c.Assert(d.Kind(), Equals, types.KindNull)
				} else {
					c.Assert(d.GetInt64(), Equals, t.expected)
				}
			}
		}
	}

	_, err := funcs[ast.Length].getFunction(s.ctx, []Expression{Zero})
	c.Assert(err, IsNil)
}

func (s *testEvaluatorSuite) TestStrcmp(c *C) {
	cases := []struct {
		args   []interface{}
		isNil  bool
		getErr bool
		res    int64
	}{
		{[]interface{}{"123", "123"}, false, false, 0},
		{[]interface{}{"123", "1"}, false, false, 1},
		{[]interface{}{"1", "123"}, false, false, -1},
		{[]interface{}{"123", "45"}, false, false, -1},
		{[]interface{}{123, "123"}, false, false, 0},
		{[]interface{}{"12.34", 12.34}, false, false, 0},
		{[]interface{}{nil, "123"}, true, false, 0},
		{[]interface{}{"123", nil}, true, false, 0},
		{[]interface{}{"", "123"}, false, false, -1},
		{[]interface{}{"123", ""}, false, false, 1},
		{[]interface{}{"", ""}, false, false, 0},
		{[]interface{}{"", nil}, true, false, 0},
		{[]interface{}{nil, ""}, true, false, 0},
		{[]interface{}{nil, nil}, true, false, 0},
		{[]interface{}{"123", errors.New("must err")}, false, true, 0},
	}
	for _, t := range cases {
		f, err := newFunctionForTest(s.ctx, ast.Strcmp, s.primitiveValsToConstants(t.args)...)
		c.Assert(err, IsNil)
		d, err := f.Eval(chunk.Row{})
		if t.getErr {
			c.Assert(err, NotNil)
		} else {
			c.Assert(err, IsNil)
			if t.isNil {
				c.Assert(d.Kind(), Equals, types.KindNull)
			} else {
				c.Assert(d.GetInt64(), Equals, t.res)
			}
		}
	}
}

func (s *testEvaluatorSuite) TestTFIDFScore(c *C) {
	fmt.Println(stringutil.TFIDFScore("数据库系统", "数据库系统概念"))
	fmt.Println(stringutil.TFIDFScore("数据库系统", "跟鸟哥学Linux"))

	fmt.Println(stringutil.TFIDFScore("2022年4月23日，南京工程高等职业技术学校一学生被骗，嫌疑人通过微信冒充受害人同学，对方以为咖啡店充值返利5倍为由诱骗受害人使用用支付宝扫码的方式转账，后发现被骗，损失1200元",
		"2022年4月24日，江苏经贸职业技术学院一学生被骗，嫌疑人在“交易猫”网站上发布出售“元神”游戏账号信息，受害人通过QQ联系对方，后对方发送陌生交易链接给受害人，诱导受害人点击该链接脱离平台交易，再以异地付款资金冻结为由，诱骗受害人通过自己支付宝向对方转账，后发现被骗，损失2000元"))
	fmt.Println(stringutil.TFIDFScore("2022年4月23日，南京工程高等职业技术学校一学生被骗，嫌疑人通过微信冒充受害人同学，对方以为咖啡店充值返利5倍为由诱骗受害人使用用支付宝扫码的方式转账，后发现被骗，损失1200元",
		"通知，为更好服务大学生高质量就业，助力县区经济和产业发展。今年新增直播荐岗县区专场，首场活动“百校千企万岗”2022年江苏省大学生就业帮扶“送岗直通车”直播荐岗活动南京六合（智能制造）专场线上直播时间为4月28日（明天）14:30开始，届时有15家优质企业提供约400个岗位，请2022届、2023届毕业生及时收看，详情参见江苏共青团微信推送。谢谢！"))
}
