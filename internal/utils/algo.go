package utils

import (
	"time"

	"math/rand"
)

func Shuffle[T any](slice []T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // 局部随机数生成器
	n := len(slice)
	for i := n - 1; i > 0; i-- {
		j := r.Intn(i + 1)                      // 使用局部随机数生成器
		slice[i], slice[j] = slice[j], slice[i] // 交换元素
	}
}
