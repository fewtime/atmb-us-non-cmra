package main

import (
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Address struct {
	Title, Price, Street, City, State, Zip, Link, RDI, CMRA string
}

func getState() []string {
	log.Println("正在获取州信息")
	url := "https://www.anytimemailbox.com/locations"

	// 发起 HTTP GET 请求
	client := &http.Client{
		Timeout: time.Second * 30,
	}
	res, err := client.Get(url)
	if err != nil {
		log.Println("请求失败: ", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Println("getState 退出错误: ", err)
		}
	}()

	if res.StatusCode != 200 {
		log.Printf("请求错误: 状态码 %d %s\n", res.StatusCode, res.Status)
	}

	// 将 HTML 响应体加载到 goquery document 中
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Println("解析 HTML 失败: ", err)
	}

	var states []string

	// 使用 CSS 选择器查找所有 href 以 "/l/usa/" 开头的 <a> 标签
	// 这是定位州链接最可靠的方法
	selector := `a[href^="/l/usa/"]`
	doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		// 提取链接的文本并添加到切片中
		stateName := s.Text()
		states = append(states, stateName)
	})

	uniqueStateMap := make(map[string]bool)
	for _, state := range states {
		uniqueStateMap[state] = true
	}

	uniqueStates := make([]string, 0, len(uniqueStateMap))
	for state := range uniqueStateMap {
		uniqueStates = append(uniqueStates, state)
	}

	sort.Strings(uniqueStates)
	log.Println("获取州信息完毕")
	return uniqueStates
}

func getStateDetail(state string) []Address {
	var parsedAddresses []Address

	log.Printf("正在获取 %s 详细信息\n", state)
	// 目标 URL
	url := "https://www.anytimemailbox.com/l/usa/" + state

	// 发起 HTTP GET 请求
	client := &http.Client{
		Timeout: time.Second * 30,
	}
	res, err := client.Get(url)
	if err != nil {
		log.Println("请求失败: ", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Println("getStateDetail 退出错误: ", err)
		}
	}()

	// 确保请求成功
	if res.StatusCode != 200 {
		log.Printf("请求错误: 状态码 %d %s\n", res.StatusCode, res.Status)
	}

	// 将 HTML 响应体加载到 goquery document 中
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Println("解析 HTML 失败: ", err)
	}

	priceRe := regexp.MustCompile(`\d+\.\d+`)
	streetRe := regexp.MustCompile(`(?i)(.*?)\s*<br\s*/?>\s*(.*?),?\s*([A-Z]{2})\s+(\d{5})`)

	// 查找所有包含地址信息的卡片元素
	doc.Find(".theme-location-item").Each(func(i int, s *goquery.Selection) {
		// 在卡片内提取城市、州和邮编所在的行
		title := s.Find("h3.t-title").Text()

		price := s.Find("div.t-price>b").Text()
		price = priceRe.FindString(price)

		streetAddress, err := s.Find("div.t-addr").Html()
		if err != nil {
			log.Println("提取地址失败: ", err)
		}

		streetMatch := streetRe.FindStringSubmatch(streetAddress)
		street := strings.TrimSpace(streetMatch[1])
		city := strings.TrimSpace(streetMatch[2])
		state := strings.TrimSpace(streetMatch[3])
		zip := strings.TrimSpace(streetMatch[4])

		link := "https://www.anytimemailbox.com" + s.Find("a").AttrOr("href", "")

		addr := Address{
			Title:  title,
			Price:  price,
			Street: street,
			City:   city,
			State:  state,
			Zip:    zip,
			Link:   link,
			RDI:    "UNKNOWN",
			CMRA:   "UNKNOWN",
		}
		parsedAddresses = append(parsedAddresses, addr)

	})

	log.Printf("获取 %s 详细信息完毕，共有 %d 个地址\n", state, len(parsedAddresses))
	return parsedAddresses
}
