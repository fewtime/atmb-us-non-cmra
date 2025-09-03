package main

import (
	"log"
	"sync"
)

const (
	configFilename   = "config.json"
	numScrapyWorkers = 10
	numATMBWorkers   = 5
)

func main() {
	// --- 1. 加载并去重州列表 ---
	states := getState()
	log.Printf("已加载 %d 个唯一的州进行抓取。", len(states))

	// --- 2. 加载初始API凭证 (无需检查数量) ---
	loadedCredentials, err := loadCredentialsFromFile(configFilename)
	if err != nil {
		log.Fatalf("读取配置文件 %s 时出错: %v", configFilename, err)
	}
	log.Printf("从 %s 中成功加载 %d 组凭证。", configFilename, len(loadedCredentials))

	apiManager := NewAPIManager(loadedCredentials)

	// --- 3. 设置 Channels 和 WaitGroups ---
	stateChan := make(chan string, len(states))
	jobs := make(chan *Address, 1000)
	results := make(chan *Address, 1000)
	failedJobs := make(chan *Address, 1000)

	var atmbWg, scrapyWg, csvWriterWg sync.WaitGroup

	// --- 4. 启动地址处理工作单元 (Smarty Workers) ---
	scrapyWg.Add(numScrapyWorkers)
	for w := 1; w <= numScrapyWorkers; w++ {
		go smartyWorker(w, apiManager, jobs, results, failedJobs, &scrapyWg)
	}

	// --- 5. 启动抓取工作单元 (ATMB Workers) ---
	atmbWg.Add(numATMBWorkers)
	for w := 1; w <= numATMBWorkers; w++ {
		go atmbWorker(w, stateChan, jobs, &atmbWg)
	}

	// --- 6. 分发抓取任务 ---
	log.Println("正在分发州名给抓取工作单元...")
	for _, state := range states {
		stateChan <- state
	}
	close(stateChan)

	// --- 7. 管理 Channel 关闭 (核心改动) ---
	var shutdownOnce sync.Once
	// 定义一个函数，用于触发关闭流程，sync.Once 会保证它只被执行一次
	initiateShutdown := func() {
		log.Println("检测到关闭信号。关闭 jobs 通道，停止接收新任务。")
		close(jobs)
	}

	// 启动一个goroutine，等待抓取完成，然后触发关闭
	go func() {
		atmbWg.Wait()
		log.Println("所有抓取工作单元已完成。")
		shutdownOnce.Do(initiateShutdown)
	}()

	// 启动另一个goroutine，等待凭证耗尽的信号，然后触发关闭
	go func() {
		// 这会阻塞，直到有 worker 向 failedJobs channel 发送数据
		<-failedJobs
		log.Println("检测到凭证耗尽信号。")
		shutdownOnce.Do(initiateShutdown)
	}()

	// 启动并发写入CSV文件 (无变化)
	csvWriterWg.Add(1)
	go func() {
		defer csvWriterWg.Done()
		writeToCSV("results.csv", results)
	}()

	// --- 8. 等待所有任务完成 ---
	log.Println("正在等待所有地址处理工作单元完成...")
	scrapyWg.Wait()
	log.Println("所有地址处理工作单元已完成。关闭 results 通道。")
	close(results)
	close(failedJobs) // 在所有 processor 都退出后，关闭 failedJobs channel

	// --- 将失败的任务写入CSV ---
	writeFailedToCSV("failed_results.csv", failedJobs)

	// 等待CSV写入完成
	csvWriterWg.Wait()

	// --- 9. 将更新后的凭证列表保存回文件 ---
	log.Println("正在将更新后的凭证列表保存回 config.json...")
	finalCredentials := apiManager.GetAllCredentials()
	if err := saveCredentialsToFile(configFilename, finalCredentials); err != nil {
		log.Printf("警告: 无法将新凭证保存到 %s: %v", configFilename, err)
	} else {
		log.Printf("已成功将 %d 组凭证保存到 %s。", len(finalCredentials), configFilename)
	}

	log.Println("程序完成。")
}
