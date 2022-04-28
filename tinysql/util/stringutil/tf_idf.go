package stringutil

import (
	"github.com/pingcap/tidb/parser"
)

func TFIDFScore(left, right string) float64 {
	var topK = 10
	leftWeight := parser.Jieba.ExtractWithWeight(left, topK)
	rightWeight := parser.Jieba.ExtractWithWeight(right, topK)
	// construct right word-weight map
	rightMap := make(map[string]float64, 10)
	for i := 0; i < len(rightWeight); i++ {
		rightMap[rightWeight[i].Word] = rightWeight[i].Weight
	}
	// calculate the similarity value: similarity = TF1*IDF1 + TF2*IDF2+***+TFn*IDFn
	// We just use jieba tf-idf res as the TFn*Idfn
	var res float64
	for i := 0; i < len(leftWeight); i++ {
		res += rightMap[leftWeight[i].Word] / float64(len(right))
	}
	return res
}
