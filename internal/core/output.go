package core

import (
	"fmt"
	"sync"
)

// OutputMutex 全局输出互斥锁，用于保护所有标准输出操作
// 防止多个goroutine的输出与动态UI渲染相互干扰
var OutputMutex sync.Mutex

// SafePrintf 线程安全的Printf封装
// 通过OutputMutex确保输出操作的原子性，避免与UI渲染冲突
func SafePrintf(format string, a ...interface{}) {
	OutputMutex.Lock()
	defer OutputMutex.Unlock()
	fmt.Printf(format, a...)
}

// SafePrintln 线程安全的Println封装
// 通过OutputMutex确保输出操作的原子性，避免与UI渲染冲突
func SafePrintln(a ...interface{}) {
	OutputMutex.Lock()
	defer OutputMutex.Unlock()
	fmt.Println(a...)
}

// SafePrint 线程安全的Print封装
// 通过OutputMutex确保输出操作的原子性，避免与UI渲染冲突
func SafePrint(a ...interface{}) {
	OutputMutex.Lock()
	defer OutputMutex.Unlock()
	fmt.Print(a...)
}
