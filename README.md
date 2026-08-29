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

## ⚠️ 95th百分位算法说明

**本工具的95th计算与Cacti完全一致。**

### 算法规则

Cacti的95th百分位计费基于 **traffic_out（出方向）** 数据，不是取流入流出的最大值。

XML模板中的 `|95:bits:0:max:2|` 含义：
- `95` = 95th百分位
- `bits` = 单位转换（字节→比特）
- `max` = 图表显示样式（非计算方式）
- `2` = **第2个数据源（traffic_out）**

### 计算公式

```
rank = int(n × 0.05) + 2

其中：
- n = 有效数据行数（total2 > 0 的行）
- 排序方式：按traffic_out值从高到低
- 取第rank名的值作为95th计费值
```

### 验证结果

| CSV文件 | n | rank | 计算结果 | Cacti显示 |
|---------|---|------|---------|----------|
| 42G | 8374 | 420 | 26.18 Gbps | 26.18 ✅ |
| 120G | 8374 | 420 | 40.97 Gbps | 40.97 ✅ |

### 月95th

月95th基于理论采样数计算（假设每5分钟一个采样点）：
```
月rank = int(月天数 × 288 × 0.05) + 2
```

如果实际数据量 ≥ 月rank，使用理论rank；否则退回使用实际数据的rank。

## 安装运行

### 方式一：直接运行

```bash
# 编译
go build -o CactiP95 main.go

# 运行
./CactiP95
```

访问 http://localhost:8888

### 方式二：系统服务（推荐）

#### 1. 部署二进制文件

```bash
sudo mkdir -p /opt/CactiP95
sudo cp CactiP95 /opt/CactiP95/
sudo chmod +x /opt/CactiP95/CactiP95
```

#### 2. 创建系统服务

```bash
sudo tee /etc/systemd/system/cactip95.service << 'EOF'
[Unit]
Description=CactiP95 - Cacti P95流量分析系统
After=network.target

[Service]
Type=simple
ExecStart=/opt/CactiP95/CactiP95
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
WorkingDirectory=/opt/CactiP95

[Install]
WantedBy=multi-user.target
EOF
```

#### 3. 启用并启动服务

```bash
sudo systemctl daemon-reload
sudo systemctl enable cactip95
sudo systemctl start cactip95
```

#### 4. 常用命令

```bash
# 查看服务状态
sudo systemctl status cactip95

# 查看日志
sudo journalctl -u cactip95 -f

# 重启服务
sudo systemctl restart cactip95

# 停止服务
sudo systemctl stop cactip95

# 禁用开机自启
sudo systemctl disable cactip95
```

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
