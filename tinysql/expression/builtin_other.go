// Copyright 2016 PingCAP, Inc.
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
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/parser/mysql"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tidb/util/stringutil"
	"github.com/pingcap/tipb/go-tipb"
)

var (
	_ functionClass = &inFunctionClass{}
	_ functionClass = &rowFunctionClass{}
	_ functionClass = &setVarFunctionClass{}
	_ functionClass = &getVarFunctionClass{}
	_ functionClass = &valuesFunctionClass{}
)

var (
	_ builtinFunc = &builtinInIntSig{}
	_ builtinFunc = &builtinInStringSig{}
	_ builtinFunc = &builtinInRealSig{}
	_ builtinFunc = &builtinRowSig{}
	_ builtinFunc = &builtinSetVarSig{}
	_ builtinFunc = &builtinGetVarSig{}
	_ builtinFunc = &builtinValuesIntSig{}
	_ builtinFunc = &builtinValuesRealSig{}
	_ builtinFunc = &builtinValuesStringSig{}
)

type inFunctionClass struct {
	baseFunctionClass
}

func (c *inFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (sig builtinFunc, err error) {
	if err := c.verifyArgs(args); err != nil {
		return nil, err
	}
	argTps := make([]types.EvalType, len(args))
	for i := range args {
		argTps[i] = args[0].GetType().EvalType()
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETInt, argTps...)
	bf.tp.Flen = 1
	switch args[0].GetType().EvalType() {
	case types.ETInt:
		sig = &builtinInIntSig{baseBuiltinFunc: bf}
		sig.setPbCode(tipb.ScalarFuncSig_InInt)
	case types.ETString:
		sig = &builtinInStringSig{baseBuiltinFunc: bf}
		sig.setPbCode(tipb.ScalarFuncSig_InString)
	case types.ETReal:
		sig = &builtinInRealSig{baseBuiltinFunc: bf}
		sig.setPbCode(tipb.ScalarFuncSig_InReal)
	}
	return sig, nil
}

// builtinInIntSig see https://dev.mysql.com/doc/refman/5.7/en/comparison-operators.html#function_in
type builtinInIntSig struct {
	baseBuiltinFunc
}

func (b *builtinInIntSig) Clone() builtinFunc {
	newSig := &builtinInIntSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinInIntSig) evalInt(row chunk.Row) (int64, bool, error) {
	arg0, isNull0, err := b.args[0].EvalInt(b.ctx, row)
	if isNull0 || err != nil {
		return 0, isNull0, err
	}
	isUnsigned0 := mysql.HasUnsignedFlag(b.args[0].GetType().Flag)
	var hasNull bool
	for _, arg := range b.args[1:] {
		evaledArg, isNull, err := arg.EvalInt(b.ctx, row)
		if err != nil {
			return 0, true, err
		}
		if isNull {
			hasNull = true
			continue
		}
		isUnsigned := mysql.HasUnsignedFlag(arg.GetType().Flag)
		if isUnsigned0 && isUnsigned {
			if evaledArg == arg0 {
				return 1, false, nil
			}
		} else if !isUnsigned0 && !isUnsigned {
			if evaledArg == arg0 {
				return 1, false, nil
			}
		} else if !isUnsigned0 && isUnsigned {
			if arg0 >= 0 && evaledArg == arg0 {
				return 1, false, nil
			}
		} else {
			if evaledArg >= 0 && evaledArg == arg0 {
				return 1, false, nil
			}
		}
	}
	return 0, hasNull, nil
}

// builtinInStringSig see https://dev.mysql.com/doc/refman/5.7/en/comparison-operators.html#function_in
type builtinInStringSig struct {
	baseBuiltinFunc
}

func (b *builtinInStringSig) Clone() builtinFunc {
	newSig := &builtinInStringSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinInStringSig) evalInt(row chunk.Row) (int64, bool, error) {
	arg0, isNull0, err := b.args[0].EvalString(b.ctx, row)
	if isNull0 || err != nil {
		return 0, isNull0, err
	}
	var hasNull bool
	for _, arg := range b.args[1:] {
		evaledArg, isNull, err := arg.EvalString(b.ctx, row)
		if err != nil {
			return 0, true, err
		}
		if isNull {
			hasNull = true
			continue
		}
		if arg0 == evaledArg {
			return 1, false, nil
		}
	}
	return 0, hasNull, nil
}

type builtinCutlStringSig struct {
	baseBuiltinFunc
}

func (b *builtinCutlStringSig) Clone() builtinFunc {
	newSig := &builtinCutlStringSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinCutlStringSig) evalInt(row chunk.Row) (int64, bool, error) {
	arg0, isNull0, err := b.args[0].EvalString(b.ctx, row)
	if isNull0 || err != nil {
		return 0, isNull0, err
	}
	var hasNull bool
	for _, arg := range b.args[1:] {
		evaledArg, isNull, err := arg.EvalString(b.ctx, row)
		if err != nil {
			return 0, true, err
		}
		if isNull {
			hasNull = true
			continue
		}
		if arg0 == evaledArg {
			return 1, false, nil
		}
	}

	return 0, hasNull, nil
}

// builtinInRealSig see https://dev.mysql.com/doc/refman/5.7/en/comparison-operators.html#function_in
type builtinInRealSig struct {
	baseBuiltinFunc
}

func (b *builtinInRealSig) Clone() builtinFunc {
	newSig := &builtinInRealSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinInRealSig) evalInt(row chunk.Row) (int64, bool, error) {
	arg0, isNull0, err := b.args[0].EvalReal(b.ctx, row)
	if isNull0 || err != nil {
		return 0, isNull0, err
	}
	var hasNull bool
	for _, arg := range b.args[1:] {
		evaledArg, isNull, err := arg.EvalReal(b.ctx, row)
		if err != nil {
			return 0, true, err
		}
		if isNull {
			hasNull = true
			continue
		}
		if arg0 == evaledArg {
			return 1, false, nil
		}
	}
	return 0, hasNull, nil
}

type rowFunctionClass struct {
	baseFunctionClass
}

func (c *rowFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (sig builtinFunc, err error) {
	if err = c.verifyArgs(args); err != nil {
		return nil, err
	}
	argTps := make([]types.EvalType, len(args))
	for i := range argTps {
		argTps[i] = args[i].GetType().EvalType()
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, argTps...)
	sig = &builtinRowSig{bf}
	return sig, nil
}

type builtinRowSig struct {
	baseBuiltinFunc
}

func (b *builtinRowSig) Clone() builtinFunc {
	newSig := &builtinRowSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString rowFunc should always be flattened in expression rewrite phrase.
func (b *builtinRowSig) evalString(row chunk.Row) (string, bool, error) {
	panic("builtinRowSig.evalString() should never be called.")
}

type setVarFunctionClass struct {
	baseFunctionClass
}

func (c *setVarFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (sig builtinFunc, err error) {
	if err = c.verifyArgs(args); err != nil {
		return nil, err
	}
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString, types.ETString)
	bf.tp.Flen = args[1].GetType().Flen
	// TODO: we should consider the type of the argument, but not take it as string for all situations.
	sig = &builtinSetVarSig{bf}
	return sig, err
}

type builtinSetVarSig struct {
	baseBuiltinFunc
}

func (b *builtinSetVarSig) Clone() builtinFunc {
	newSig := &builtinSetVarSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinSetVarSig) evalString(row chunk.Row) (res string, isNull bool, err error) {
	var varName string
	sessionVars := b.ctx.GetSessionVars()
	varName, isNull, err = b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", isNull, err
	}
	res, isNull, err = b.args[1].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", isNull, err
	}
	varName = strings.ToLower(varName)
	sessionVars.UsersLock.Lock()
	sessionVars.Users[varName] = stringutil.Copy(res)
	sessionVars.UsersLock.Unlock()
	return res, false, nil
}

type getVarFunctionClass struct {
	baseFunctionClass
}

func (c *getVarFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (sig builtinFunc, err error) {
	if err = c.verifyArgs(args); err != nil {
		return nil, err
	}
	// TODO: we should consider the type of the argument, but not take it as string for all situations.
	bf := newBaseBuiltinFuncWithTp(ctx, args, types.ETString, types.ETString)
	bf.tp.Flen = mysql.MaxFieldVarCharLength
	sig = &builtinGetVarSig{bf}
	return sig, nil
}

type builtinGetVarSig struct {
	baseBuiltinFunc
}

func (b *builtinGetVarSig) Clone() builtinFunc {
	newSig := &builtinGetVarSig{}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

func (b *builtinGetVarSig) evalString(row chunk.Row) (string, bool, error) {
	sessionVars := b.ctx.GetSessionVars()
	varName, isNull, err := b.args[0].EvalString(b.ctx, row)
	if isNull || err != nil {
		return "", isNull, err
	}
	varName = strings.ToLower(varName)
	sessionVars.UsersLock.RLock()
	defer sessionVars.UsersLock.RUnlock()
	if v, ok := sessionVars.Users[varName]; ok {
		return v, false, nil
	}
	return "", true, nil
}

type valuesFunctionClass struct {
	baseFunctionClass

	offset int
	tp     *types.FieldType
}

func (c *valuesFunctionClass) getFunction(ctx sessionctx.Context, args []Expression) (sig builtinFunc, err error) {
	if err = c.verifyArgs(args); err != nil {
		return nil, err
	}
	bf := newBaseBuiltinFunc(ctx, args)
	bf.tp = c.tp
	switch c.tp.EvalType() {
	case types.ETInt:
		sig = &builtinValuesIntSig{bf, c.offset}
	case types.ETReal:
		sig = &builtinValuesRealSig{bf, c.offset}
	case types.ETString:
		sig = &builtinValuesStringSig{bf, c.offset}
	}
	return sig, nil
}

type builtinValuesIntSig struct {
	baseBuiltinFunc

	offset int
}

func (b *builtinValuesIntSig) Clone() builtinFunc {
	newSig := &builtinValuesIntSig{offset: b.offset}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalInt evals a builtinValuesIntSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_values
func (b *builtinValuesIntSig) evalInt(_ chunk.Row) (int64, bool, error) {
	if !b.ctx.GetSessionVars().StmtCtx.InInsertStmt {
		return 0, true, nil
	}
	row := b.ctx.GetSessionVars().CurrInsertValues
	if row.IsEmpty() {
		return 0, true, errors.New("Session current insert values is nil")
	}
	if b.offset < row.Len() {
		if row.IsNull(b.offset) {
			return 0, true, nil
		}
		return row.GetInt64(b.offset), false, nil
	}
	return 0, true, errors.Errorf("Session current insert values len %d and column's offset %v don't match", row.Len(), b.offset)
}

type builtinValuesRealSig struct {
	baseBuiltinFunc

	offset int
}

func (b *builtinValuesRealSig) Clone() builtinFunc {
	newSig := &builtinValuesRealSig{offset: b.offset}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalReal evals a builtinValuesRealSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_values
func (b *builtinValuesRealSig) evalReal(_ chunk.Row) (float64, bool, error) {
	if !b.ctx.GetSessionVars().StmtCtx.InInsertStmt {
		return 0, true, nil
	}
	row := b.ctx.GetSessionVars().CurrInsertValues
	if row.IsEmpty() {
		return 0, true, errors.New("Session current insert values is nil")
	}
	if b.offset < row.Len() {
		if row.IsNull(b.offset) {
			return 0, true, nil
		}
		if b.getRetTp().Tp == mysql.TypeFloat {
			return float64(row.GetFloat32(b.offset)), false, nil
		}
		return row.GetFloat64(b.offset), false, nil
	}
	return 0, true, errors.Errorf("Session current insert values len %d and column's offset %v don't match", row.Len(), b.offset)
}

type builtinValuesStringSig struct {
	baseBuiltinFunc

	offset int
}

func (b *builtinValuesStringSig) Clone() builtinFunc {
	newSig := &builtinValuesStringSig{offset: b.offset}
	newSig.cloneFrom(&b.baseBuiltinFunc)
	return newSig
}

// evalString evals a builtinValuesStringSig.
// See https://dev.mysql.com/doc/refman/5.7/en/miscellaneous-functions.html#function_values
func (b *builtinValuesStringSig) evalString(_ chunk.Row) (string, bool, error) {
	if !b.ctx.GetSessionVars().StmtCtx.InInsertStmt {
		return "", true, nil
	}
	row := b.ctx.GetSessionVars().CurrInsertValues
	if row.IsEmpty() {
		return "", true, errors.New("Session current insert values is nil")
	}
	if b.offset >= row.Len() {
		return "", true, errors.Errorf("Session current insert values len %d and column's offset %v don't match", row.Len(), b.offset)
	}

	if row.IsNull(b.offset) {
		return "", true, nil
	}

	// Specially handle the ENUM/SET/BIT input value.
	if retType := b.getRetTp(); retType.Hybrid() {
		val := row.GetDatum(b.offset, retType)
		res, err := val.ToString()
		return res, err != nil, err
	}

	return row.GetString(b.offset), false, nil
}
