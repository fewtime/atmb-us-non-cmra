package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"time"
)

// writeToCSV 将成功处理的地址写入CSV文件。
// 它具有强大的容错机制：
// 1. 尝试写入指定的主文件。
// 2. 如果失败，则尝试写入一个带时间戳的备用文件。
// 3. 如果再次失败，则将所有数据打印到控制台，以防丢失。
func writeToCSV(filename string, results <-chan *Address) {
	// --- 1. 缓冲结果 ---
	// 为了能够在写入失败时进行重试或回退，我们需要先将 channel 中的所有结果收集到内存中。
	// 注意：这会增加内存使用量。如果结果集非常巨大，可能需要更复杂的流式处理策略。
	var addresses []*Address
	for addr := range results {
		addresses = append(addresses, addr)
	}

	// 如果没有结果，则直接返回，无需创建空文件。
	if len(addresses) == 0 {
		log.Println("没有需要写入CSV的结果。")
		return
	}

	log.Printf("所有地址处理完毕。准备将 %d 条结果写入CSV文件...", len(addresses))

	// --- 2. 抽象写入逻辑 ---
	// 我们定义一个可复用的写入函数，以避免代码重复。
	writerFunc := func(f *os.File) error {
		writer := csv.NewWriter(f)
		defer writer.Flush()

		header := []string{"Title", "Price", "Street", "City", "State", "Zip", "Link", "CMRA", "RDI"}
		if err := writer.Write(header); err != nil {
			return fmt.Errorf("写入CSV表头失败: %w", err)
		}

		for _, addr := range addresses {
			record := []string{
				addr.Title, addr.Price, addr.Street, addr.City,
				addr.State, addr.Zip, addr.Link, addr.CMRA, addr.RDI,
			}
			if err := writer.Write(record); err != nil {
				// 记录单行写入错误，但不中断整个过程
				log.Printf("警告: 写入记录到CSV时发生错误: %s", err)
			}
		}
		return writer.Error() // 返回 writer 可能遇到的任何异步错误
	}

	// --- 3. 执行写入操作（首选 + 备用 + 最终方案） ---
	// 首选方案
	file, err := os.Create(filename)
	if err == nil {
		defer func() {
			if err := file.Close(); err != nil {
				log.Println("writeToCSV 正常文件退出错误: ", err)
			}
		}()

		log.Printf("正在写入主文件: %s", filename)
		if err := writerFunc(file); err == nil {
			log.Printf("结果已成功写入 %s 文件。", filename)
			return
		}
		log.Printf("错误: 写入主文件 %s 时失败: %v", filename, err)
	}

	// 如果首选方案失败，记录警告并尝试备用方案
	log.Printf("警告: 无法创建主文件 '%s' (%v)。正在尝试创建备用文件...", filename, err)
	fallbackFilename := fmt.Sprintf("results_fallback_%s.csv", time.Now().Format("20060102150405"))

	fallbackFile, fallbackErr := os.Create(fallbackFilename)
	if fallbackErr == nil {
		defer func() {
			if err := fallbackFile.Close(); err != nil {
				log.Println("writeToCSV 备份文件退出错误: ", err)
			}
		}()

		log.Printf("正在写入备用文件: %s", fallbackFilename)
		if err := writerFunc(fallbackFile); err == nil {
			log.Printf("结果已成功写入备用文件 %s。", fallbackFilename)
			return
		}
		log.Printf("错误: 写入备用文件 %s 时也失败了: %v", fallbackFilename, err)
	}

	// 如果备用方案也失败了，执行最终方案
	log.Println("!!严重警告!! 文件写入彻底失败。为防止数据丢失，将把所有结果打印到控制台。")
	log.Println("--- 数据开始 ---")
	// 打印一个简易的CSV格式到日志
	fmt.Println("Title,Price,Street,City,State,Zip,Link,CMRA,RDI")
	for _, addr := range addresses {
		fmt.Printf("%q,%q,%q,%q,%q,%q,%q,%q,%q\n",
			addr.Title, addr.Price, addr.Street, addr.City,
			addr.State, addr.Zip, addr.Link, addr.CMRA, addr.RDI,
		)
	}
	log.Println("--- 数据结束 ---")
}

// writeFailedToCSV 用于将因凭证耗尽等原因未能处理的任务写入CSV文件。
// 为简洁起见，此函数使用了较为直接的错误处理方式
func writeFailedToCSV(filename string, failedJobs <-chan *Address) {
	// 将 channel 中剩余的任务收集起来
	var failedAddresses []*Address
	for addr := range failedJobs {
		failedAddresses = append(failedAddresses, addr)
	}

	// 如果 channel 中没有数据，则不创建文件
	if len(failedAddresses) == 0 {
		return
	}

	log.Printf("检测到 %d 个处理失败的任务，正在写入 %s...", len(failedAddresses), filename)

	file, err := os.Create(filename)
	if err != nil {
		// 这里的 log.Fatalf 仍然比较严厉，可以按照 writeToCSV 的模式进行修改
		log.Fatalf("无法创建失败任务的CSV文件: %s", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Println("writeFailedToCSV 文件退出错误: ", err)
		}
	}()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	header := []string{"Title", "Price", "Street", "City", "State", "Zip", "Link", "CMRA", "RDI"}
	if err := writer.Write(header); err != nil {
		log.Fatalf("写入失败任务CSV表头失败: %s", err)
	}

	// 遍历所有失败的任务并写入
	for _, addr := range failedAddresses {
		record := []string{
			addr.Title, addr.Price, addr.Street, addr.City,
			addr.State, addr.Zip, addr.Link, addr.CMRA, addr.RDI,
		}
		if err := writer.Write(record); err != nil {
			log.Printf("写入失败记录到CSV时发生错误: %s", err)
		}
	}

	log.Printf("所有失败的任务已成功写入 %s 文件。", filename)
}
