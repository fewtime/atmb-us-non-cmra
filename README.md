# atmb-us-non-cmra

优选 [anytimemailbox](https://www.anytimemailbox.com/) 地址。

## 功能

结合 [第三方](https://www.smarty.com/) 查询接口，过滤出非 CMRA 的地址。
并按照是否为住宅地址进行排序。

运行结果保存为 `result.csv` 文件。

## 运行
1. 安装 Go (建议 1.18 或更高版本)。
2. 注册 [smarty](https://www.smarty.com/) 帐号，并获取 API Secret keys. 由于免费帐号一个月只能查询 1000 次，而 atmb 目前有 2000 多个美国地址，所以至少需要注册三个帐号来完成查询。（程序在运行过程中若发现 API 次数不够，会提示输入新 API，只需注册后输入即可）
3. 创建配置文件
在项目根目录下创建一个名为 config.json 的文件，并按以下格式填入你的 SmartyStreets API 凭证。你可以添加多组凭证。
```json
[
  {
    "auth_id": "你的_AUTH_ID",
    "auth_token": "你的_AUTH_TOKEN"
  }
]
```
4. 安装依赖
打开终端，进入项目目录，然后运行：
```bash
go mod tidy
```
5. 启动程序
运行以下命令来启动程序：
```bash
go build
./atmb
```
程序将开始抓取和处理，并将日志输出到控制台。
输出文件
`results.csv`: 包含所有成功处理的地址。
`failed_results.csv`: 包含因凭证耗尽等原因未能处理的地址。