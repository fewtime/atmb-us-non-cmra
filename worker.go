package main

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/smartystreets/smartystreets-go-sdk/wireup"
)

// 定义重试相关的常量
const (
	maxRetries     = 4               // 最大重试次数 (总共会尝试 1 + 4 = 5次)
	initialBackoff = 2 * time.Second // 初始退避时间
)

// smartyWorker 是smarty工作单元，现在包含了指数退避重试逻辑
func smartyWorker(id int, apiManager *APIManager, jobs <-chan *Address, results chan<- *Address, failedJobs chan<- *Address, wg *sync.WaitGroup) {
	defer wg.Done()

	for addr := range jobs {
		log.Printf("[Scrapy %d] 正在处理地址: %s, %s", id, addr.Street, addr.City)

		var success bool // 标记地址是否已成功处理

		// 重试循环 (最多 maxRetries + 1 次尝试)
		for attempt := 0; attempt <= maxRetries; attempt++ {
			if attempt > 0 {
				// 计算本次重试的等待时间 (2s, 4s, 8s...)
				backoffDuration := initialBackoff * time.Duration(1<<(attempt-1))
				log.Printf("[Scrapy %d] 第 %d 次尝试失败。将在 %v 后重试...", id, attempt, backoffDuration)
				time.Sleep(backoffDuration)
			}

			// 1. 获取凭证
			cred, ok := apiManager.GetCredentials()
			if !ok {
				log.Printf("[Scrapy %d] 所有API凭证均已失效，工作单元退出。\n", id)
				// 将无法处理的地址发送到 failedJobs channel
				failedJobs <- addr
				return
			}

			// 2. 发起请求
			client := wireup.BuildUSStreetAPIClient(wireup.SecretKeyCredential(cred.AuthID, cred.AuthToken))
			err := SmartyInfo(client, addr)

			// 3. 处理结果
			if err == nil {
				// 成功！将结果发送并跳出重试循环
				results <- addr
				success = true
				break
			}

			// 如果是 "地址未知" 错误，则无需重试，直接放弃这个地址，但做记录
			if errors.Is(err, ErrUnknownAddress) {
				log.Printf("[Scrapy %d] 地址未知，无需重试: %s, %s", id, addr.Street, addr.City)
				failedJobs <- addr
				success = true // 标记为"已处理"（尽管是失败的），以防止最后的放弃日志
				break
			}

			// 对于其他所有错误，记录日志，标记凭证失效，然后继续下一次重试
			log.Printf("[Scrapy %d] 使用凭证 %s 失败 (尝试 %d/%d): %v", id, cred.AuthID, attempt+1, maxRetries+1, err)
			apiManager.InvalidateCurrent()
		}

		// 如果所有重试都失败了，记录一条最终的放弃日志
		if !success {
			log.Printf("[Scrapy %d] 所有重试均失败，放弃地址: %s, %s", id, addr.Street, addr.City)
		}
	}
}

// atmbWorker 是 ATMB 抓取具体州地址的工作单位
func atmbWorker(id int, stateChan <-chan string, jobs chan<- *Address, wg *sync.WaitGroup) {
	defer wg.Done()

	for state := range stateChan {
		log.Printf("[ATMB %d] 正在抓取州: %s", id, state)

		addresses := getStateDetail(state)

		log.Printf("[ATMB %d] 在 %s 找到 %d 个地址，正在推送到处理队列...", id, state, len(addresses))

		for i := range addresses {
			jobs <- &addresses[i]
		}
	}
	log.Printf("[ATMB %d] 已完成所有任务，正在退出。", id)
}
