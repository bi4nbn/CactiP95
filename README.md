# CactiP95

Cacti®点位数据分析系统 - 基于95th百分位的流量计费分析工具

## 功能特性

- 📊 支持Cacti导出的CSV格式
- 🔍 自动检测"合计"列或使用最后两列
- 📈 计算当前95th和月95th计费值
- 🎯 点击卡片跳转到对应数据行
- 📁 支持拖拽上传文件
- 🔄 双击背景区域返回主页
- 📱 响应式设计，支持移动端

## 安装运行

```bash
# 编译
go build -o CactiP95 main.go

# 运行
./CactiP95
```

访问 http://localhost:8888

## 使用方法

1. 准备Cacti导出的CSV文件
2. 拖拽或选择文件上传
3. 系统自动分析并显示95th百分位值
4. 点击卡片可跳转到对应数据行

## CSV格式支持

- 自动识别"合计"列
- 无"合计"列时使用最后两列
- 支持BOM编码

## 技术栈

- Go语言
- Bootstrap 5
- Bootstrap Icons

## 许可证

MIT License
