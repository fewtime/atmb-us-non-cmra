package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

// ApiCredential 用于封装AuthID和AuthToken
type ApiCredential struct {
	AuthID    string `json:"auth_id"`
	AuthToken string `json:"auth_token"`
}

// APIManager 负责管理API密钥
type APIManager struct {
	credentials []ApiCredential // 存储所有API凭证
	current     int             // 当前使用的凭证索引
	usageCount  int             // 当前凭证的使用次数
	mutex       sync.Mutex      // 互斥锁，保证线程安全
	maxUsage    int             // 单个凭证的最大使用次数
}

// NewAPIManager 创建一个新的API密钥管理器
func NewAPIManager(credentials []ApiCredential) *APIManager {
	return &APIManager{
		credentials: credentials,
		current:     0,
		usageCount:  0,
		maxUsage:    1000,
	}
}

// GetCredentials 获取一个可用的API凭证。
// 如果所有凭证均已耗尽，它会暂停并请求用户输入新的凭证。
// 如果用户未能提供新凭证，它会返回 false，示意工作单元应停止工作。
func (m *APIManager) GetCredentials() (ApiCredential, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查当前凭证是否已达到使用上限，如果是，则切换到下一个
	if m.usageCount >= m.maxUsage && m.current < len(m.credentials) {
		log.Printf("凭证 %s 已达到使用上限，正在切换...\n", m.credentials[m.current].AuthID)
		m.rotate()
	}

	// 检查是否所有凭证都已用尽
	if m.current >= len(m.credentials) {
		log.Println("所有可用的API凭证均已耗尽或失效。程序已暂停，等待输入新的凭证。")

		// 动态从用户处获取新的凭证
		newCredentials := getAdditionalCredentialsFromUser(1) // 至少请求一组新的

		if len(newCredentials) == 0 {
			log.Println("用户没有提供新的凭证。处理工作将停止。")
			return ApiCredential{}, false // 这是关键的退出信号
		}

		// 将新凭证添加到管理器中
		m.credentials = append(m.credentials, newCredentials...)
		log.Printf("已成功添加 %d 组新凭证。程序将继续处理。", len(newCredentials))
		// m.current 此时正好是新凭证的索引，无需修改
	}

	cred := m.credentials[m.current]
	m.usageCount++

	return cred, true
}

// InvalidateCurrent 标记当前凭证为无效并立即切换到下一个
func (m *APIManager) InvalidateCurrent() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.current >= len(m.credentials) {
		return
	}
	log.Printf("凭证 %s 失效，正在强制切换...\n", m.credentials[m.current].AuthID)
	m.rotate()
}

// rotate 切换到下一个API凭证 (非线程安全，需要被外部调用者加锁)
func (m *APIManager) rotate() {
	m.current++
	m.usageCount = 0 // 重置计数器
}

// GetAllCredentials 安全地返回当前管理器中所有凭证的副本。
func (m *APIManager) GetAllCredentials() []ApiCredential {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	// 返回一个副本以防止外部修改
	credsCopy := make([]ApiCredential, len(m.credentials))
	copy(credsCopy, m.credentials)
	return credsCopy
}

// --- 文件和用户输入辅助函数  ---

func loadCredentialsFromFile(filename string) ([]ApiCredential, error) {
	var credentials []ApiCredential
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return credentials, nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	if len(data) == 0 {
		return credentials, nil
	}
	err = json.Unmarshal(data, &credentials)
	if err != nil {
		return nil, fmt.Errorf("解析JSON配置文件失败: %w", err)
	}
	return credentials, nil
}

func saveCredentialsToFile(filename string, credentials []ApiCredential) error {
	data, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return fmt.Errorf("格式化凭证为JSON失败: %w", err)
	}
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}

func getAdditionalCredentialsFromUser(requiredCount int) []ApiCredential {
	var credentials []ApiCredential
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Printf("\n--- 需要补充API凭证 ---\n")
	if requiredCount > 0 {
		fmt.Printf("您至少需要提供 %d 组新的API凭证才能继续。\n", requiredCount)
	}
	fmt.Println("请按提示逐个输入 Auth ID 和 Auth Token (输入空行则停止)。")

	for i := 0; ; i++ {
		fmt.Printf("\n请输入第 %d 组新凭证:\n", i+1)
		fmt.Print("  Auth ID: ")
		scanner.Scan()
		authID := strings.TrimSpace(scanner.Text())
		if authID == "" {
			log.Println("输入为空，已终止凭证添加。")
			break
		}

		fmt.Print("  Auth Token: ")
		scanner.Scan()
		authToken := strings.TrimSpace(scanner.Text())
		if authToken == "" {
			log.Println("输入为空，已终止凭证添加。")
			break
		}
		credentials = append(credentials, ApiCredential{AuthID: authID, AuthToken: authToken})
		if len(credentials) >= requiredCount {
			fmt.Println("已满足最低数量要求，您可以选择继续添加或直接按回车停止。")
		}
	}
	fmt.Println("--------------------")
	return credentials
}
