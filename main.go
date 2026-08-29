package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 流量数据结构
type TrafficData struct {
	Date   string
	Total1 float64
	Total2 float64
	Total  float64 // 取较大值
}

// 95th计算结果
type P95Result struct {
	CurrentP95      float64
	CurrentP95Rank  int
	MonthlyP95      float64
	MonthlyP95Rank  int
	MonthDays       int
	TotalSamples    int
	MaxTraffic      float64
	DataRange       string
	StartDate       string
	EndDate         string
	SortedData      []TrafficData
}

// 读取CSV文件
func readCSV(filepath string) ([]TrafficData, string, string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, "", "", err
	}
	defer file.Close()

	// 读取文件内容并去除BOM
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, "", "", err
	}
	
	// 去除BOM标记
	contentStr := strings.TrimPrefix(string(content), "\ufeff")
	
	// 按行分割
	lines := strings.Split(contentStr, "\n")
	
	// 读取元数据中的开始日期和结束日期
	var startDate, endDate string
	if len(lines) > 4 {
		// 第3行是开始日期
		startFields := parseCSVLine(lines[2])
		if len(startFields) >= 2 {
			startDate = strings.Trim(startFields[1], "\"")
		}
		// 第4行是结束日期
		endFields := parseCSVLine(lines[3])
		if len(endFields) >= 2 {
			endDate = strings.Trim(endFields[1], "\"")
		}
	}
	
	// 读取表头，找到"合计"列的位置
	var headerFields []string
	var total1Index, total2Index int
	
	// 查找表头行（第11行，索引10）
	if len(lines) > 10 {
		headerFields = parseCSVLine(lines[10])
		totalCount := 0
		for i, field := range headerFields {
			// 去除引号和空格
			cleanField := strings.TrimSpace(strings.Trim(field, "\""))
			if cleanField == "合计" || cleanField == "　合计" {
				totalCount++
				if totalCount == 1 {
					total1Index = i
				} else if totalCount == 2 {
					total2Index = i
				}
			}
		}
		
		// 如果没有找到"合计"列，使用最后两列
		if totalCount == 0 {
			total1Index = len(headerFields) - 2
			total2Index = len(headerFields) - 1
		}
	}
	
	var data []TrafficData
	
	// 从第12行开始读取数据（跳过前10行，第11行是表头）
	for i := 11; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		
		// 手动解析CSV行（处理引号）
		fields := parseCSVLine(line)
		
		if len(fields) > total2Index && strings.HasPrefix(fields[0], "2026") {
			total1, err1 := strconv.ParseFloat(fields[total1Index], 64)
			total2, err2 := strconv.ParseFloat(fields[total2Index], 64)
			
			if err1 == nil && err2 == nil && total1 > 0 && total2 > 0 {
				// 取两组数据的较大值
				maxValue := math.Max(total1, total2)
				data = append(data, TrafficData{
					Date:   fields[0],
					Total1: total1,
					Total2: total2,
					Total:  maxValue,
				})
			}
		}
	}

	return data, startDate, endDate, nil
}

// 解析CSV行（处理引号）
func parseCSVLine(line string) []string {
	var fields []string
	var current strings.Builder
	inQuotes := false
	
	for _, ch := range line {
		switch {
		case ch == '"':
			inQuotes = !inQuotes
		case ch == ',' && !inQuotes:
			fields = append(fields, strings.TrimSpace(current.String()))
			current.Reset()
		default:
			current.WriteRune(ch)
		}
	}
	
	// 添加最后一个字段
	fields = append(fields, strings.TrimSpace(current.String()))
	
	return fields
}

// 获取月份天数
func getMonthDays(dateStr string) int {
	t, err := time.Parse("2006-01-02 15:04:05", dateStr)
	if err != nil {
		return 30 // 默认30天
	}
	
	year := t.Year()
	month := t.Month()
	
	// 获取该月的天数
	if month == time.February {
		if (year%4 == 0 && year%100 != 0) || year%400 == 0 {
			return 29
		}
		return 28
	}
	
	if month == time.April || month == time.June || month == time.September || month == time.November {
		return 30
	}
	
	return 31
}

// 计算95th值
func calculateP95(data []TrafficData, startDate, endDate string) P95Result {
	if len(data) == 0 {
		return P95Result{}
	}

	// 按流量从高到低排序
	sort.Slice(data, func(i, j int) bool {
		return data[i].Total > data[j].Total
	})

	totalSamples := len(data)
	
	// 当前95th（基于实际数据）
	top5Percent := int(float64(totalSamples) * 0.05)
	currentP95Rank := top5Percent + 1
	currentP95 := data[currentP95Rank-1].Total

	// 月95th（基于整月理论采样点）
	// 假设每5分钟一个采样点，每天288个
	monthDays := getMonthDays(startDate)
	samplesPerDay := 288
	totalSamplesTheory := monthDays * samplesPerDay
	monthlyP95RankTheory := int(float64(totalSamplesTheory) * 0.05) + 1
	
	// 如果实际数据足够，用理论位置；否则用实际位置
	var monthlyP95Rank int
	if totalSamples >= monthlyP95RankTheory {
		monthlyP95Rank = monthlyP95RankTheory
	} else {
		monthlyP95Rank = int(float64(totalSamples) * 0.05) + 1
	}
	
	monthlyP95 := data[monthlyP95Rank-1].Total

	// 最大流量
	maxTraffic := data[0].Total

	return P95Result{
		CurrentP95:     currentP95,
		CurrentP95Rank: currentP95Rank,
		MonthlyP95:     monthlyP95,
		MonthlyP95Rank: monthlyP95Rank,
		MonthDays:      monthDays,
		TotalSamples:   totalSamples,
		MaxTraffic:     maxTraffic,
		DataRange:      fmt.Sprintf("%s 至 %s", startDate, endDate),
		StartDate:      startDate,
		EndDate:        endDate,
		SortedData:     data,
	}
}

// Web处理器
func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Cacti®点位数据分析系统</title>
    <link rel="icon" href="/static/logo.svg" type="image/svg+xml">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.10.0/font/bootstrap-icons.css" rel="stylesheet">
    <style>
        :root {
            --primary-color: #2F5496;
            --secondary-color: #1a3a6c;
            --accent-color: #ff6b6b;
        }
        
        * {
            box-sizing: border-box;
        }
        
        body {
            font-family: 'Segoe UI', 'Microsoft YaHei', sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 15px;
            margin: 0;
            zoom: 0.8;
        }
        
        .main-container {
            max-width: 1400px;
            margin: 0 auto;
        }
        
        .card {
            border: none;
            border-radius: 15px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.2);
            overflow: hidden;
        }
        
        .card-header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 50%, #f093fb 100%);
            color: white;
            padding: 20px;
            border-bottom: none;
            position: relative;
            overflow: hidden;
        }
        
        .card-header::before {
            content: '';
            position: absolute;
            top: -50%;
            left: -50%;
            width: 200%;
            height: 200%;
            background: radial-gradient(circle, rgba(255,255,255,0.1) 0%, transparent 60%);
            animation: shimmer 8s ease-in-out infinite;
        }
        
        @keyframes shimmer {
            0%, 100% { transform: translate(0, 0); }
            50% { transform: translate(10%, 10%); }
        }
        
        .card-header h1 {
            margin: 0;
            font-weight: 700;
            font-size: 1.5rem;
        }
        
        .card-body {
            padding: 20px;
        }
        
        /* 上传区域 */
        .upload-section {
            background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%);
            border-radius: 15px;
            padding: 30px 20px;
            text-align: center;
            border: 3px dashed #dee2e6;
            transition: all 0.3s ease;
            cursor: pointer;
            position: relative;
        }
        
        .upload-section:hover,
        .upload-section.drag-over {
            border-color: var(--primary-color);
            background: linear-gradient(135deg, #e3f2fd 0%, #bbdefb 100%);
            transform: scale(1.02);
        }
        
        .upload-section.drag-over {
            border-color: #28a745;
            background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%);
        }
        
        .upload-icon {
 font-size: 3rem;
            color: var(--primary-color);
            margin-bottom: 15px;
        }
        
        .upload-title {
            font-size: 1.3rem;
            font-weight: 600;
            color: #333;
            margin-bottom: 8px;
        }
        
        .upload-subtitle {
            color: #666;
            margin-bottom: 20px;
            font-size: 0.9rem;
        }
        
        .upload-hint {
            color: #999;
 font-size: 0.8rem;
            margin-top: 10px;
        }
        
        .file-input {
            display: none;
        }
        
        .file-name {
            margin-top: 15px;
            padding: 10px 15px;
            background: white;
            border-radius: 8px;
            display: none;
            align-items: center;
            justify-content: center;
            gap: 10px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }
        
        .file-name.show {
            display: flex;
        }
        
        .file-name i {
            color: #28a745;
            font-size: 1.2rem;
        }
        
        .file-name span {
            font-weight: 500;
            color: #333;
            word-break: break-all;
        }
        
        .btn-primary {
            background: linear-gradient(135deg, var(--primary-color) 0%, var(--secondary-color) 100%);
            border: none;
            padding: 12px 35px;
            border-radius: 25px;
            font-weight: 600;
            font-size: 1rem;
            transition: all 0.3s ease;
            margin-top: 15px;
        }
        
        .btn-primary:hover {
            transform: translateY(-2px);
            box-shadow: 0 5px 20px rgba(47, 84, 150, 0.4);
        }
        
        .btn-primary:disabled {
            opacity: 0.6;
            cursor: not-allowed;
            transform: none;
            box-shadow: none;
        }
        
        /* 功能特性 */
        .features {
            margin-top: 30px;
        }
        
        .feature-item {
            text-align: center;
            padding: 15px;
            margin-bottom: 15px;
        }
        
        .feature-icon {
            font-size: 2rem;
            color: var(--primary-color);
            margin-bottom: 10px;
        }
        
        .feature-title {
            font-weight: 600;
            color: #333;
            margin-bottom: 5px;
            font-size: 0.95rem;
        }
        
        .feature-desc {
            color: #666;
            font-size: 0.85rem;
        }
        
        /* 加载动画 */
        .loading-overlay {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(0,0,0,0.7);
            z-index: 9999;
            justify-content: center;
            align-items: center;
        }
        
        .loading-overlay.show {
            display: flex;
        }
        
        .loading-content {
            background: white;
            padding: 30px 40px;
            border-radius: 15px;
            text-align: center;
        }
        
        .spinner {
            width: 50px;
            height: 50px;
            border: 4px solid #f3f3f3;
            border-top: 4px solid var(--primary-color);
            border-radius: 50%;
            animation: spin 1s linear infinite;
            margin: 0 auto 15px;
        }
        
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
        
        @keyframes pulse {
            0% { 
                box-shadow: 0 0 0 0 rgba(47, 84, 150, 0.3);
                transform: scale(1);
            }
            25% { 
                box-shadow: 0 0 15px 5px rgba(47, 84, 150, 0.2);
                transform: scale(1.005);
            }
            50% { 
                box-shadow: 0 0 25px 8px rgba(47, 84, 150, 0.15);
                transform: scale(1);
            }
            75% { 
                box-shadow: 0 0 15px 5px rgba(47, 84, 150, 0.2);
                transform: scale(1.005);
            }
            100% { 
                box-shadow: 0 0 0 0 rgba(47, 84, 150, 0);
                transform: scale(1);
            }
        }
        
        @keyframes highlightPulse {
            0% { background-position: 0% 50%; }
            50% { background-position: 100% 50%; }
            100% { background-position: 0% 50%; }
        }
        
        .row-highlight {
            animation: pulse 2s ease-in-out;
            position: relative;
        }
        
        .row-highlight::after {
            content: '';
            position: absolute;
            left: 0;
 right: 0;
            top: 0;
            bottom: 0;
            border: 2px solid var(--primary-color);
            border-radius: 4px;
            animation: borderPulse 1.5s ease-in-out;
            pointer-events: none;
        }
        
        @keyframes borderPulse {
            0% { opacity: 0; transform: scaleX(0.8); }
            50% { opacity: 1; transform: scaleX(1.02); }
            100% { opacity: 0; transform: scaleX(1); }
        }
        
        @keyframes slideIn {
            0% { opacity: 0; transform: translateX(-20px); }
            100% { opacity: 1; transform: translateX(0); }
        }
        
        .highlight-row {
            animation: slideIn 0.3s ease-out;
        }
        
        /* 响应式设计 */
        @media (min-width: 768px) {
            body {
                padding: 30px;
            }
            
            .card-header {
                padding: 25px 30px;
            }
            
            .card-header h1 {
                font-size: 2rem;
            }
            
            .card-body {
                padding: 40px;
            }
            
            .upload-section {
                padding: 40px;
            }
            
            .upload-icon {
                font-size: 4rem;
            }
            
            .upload-title {
                font-size: 1.5rem;
            }
        }
        
        @media (max-width: 767px) {
            .features .row {
                flex-direction: column;
            }
            
            .feature-item {
                padding: 10px;
            }
        }
    </style>
</head>
<body>
    <!-- 加载动画 -->
    <div class="loading-overlay" id="loadingOverlay">
        <div class="loading-content">
            <div class="spinner"></div>
            <p class="mb-0">正在分析数据，请稍候...</p>
        </div>
    </div>
    
    <div class="main-container">
        <div class="card">
            <div class="card-header text-center">
                <h1><img src="/static/logo.svg" alt="Cacti" style="height:36px;vertical-align:middle;margin-right:10px;">Cacti®点位数据分析系统</h1>
            </div>
            <div class="card-body">
                <form action="/upload" method="post" enctype="multipart/form-data" id="uploadForm">
                    <div class="upload-section" id="dropZone">
                        <div class="upload-icon">
                            <i class="bi bi-cloud-upload"></i>
                        </div>
                        <h3 class="upload-title">上传CSV文件</h3>
                        <p class="upload-subtitle">支持Cacti导出的CSV格式，自动解析并计算95th值</p>
                        
                        <input type="file" class="file-input" id="fileInput" name="file" accept=".csv" required>
                        
                        <button type="button" class="btn btn-outline-primary" id="selectBtn">
                            <i class="bi bi-folder2-open me-2"></i>选择文件
                        </button>
                        
                        <p class="upload-hint">
                            <i class="bi bi-info-circle me-1"></i>
                            <span class="d-none d-md-inline">或将文件拖拽到此处</span>
                            <span class="d-md-none">点击上方按钮选择文件</span>
                        </p>
                        
                        <div class="file-name" id="fileName">
                            <i class="bi bi-file-earmark-check"></i>
                            <span id="fileNameText"></span>
                        </div>
                    </div>
                </form>
            </div>
        </div>
    </div>
    
    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>
    <script>
        const dropZone = document.getElementById('dropZone');
        const fileInput = document.getElementById('fileInput');
        const selectBtn = document.getElementById('selectBtn');
        const fileName = document.getElementById('fileName');
        const fileNameText = document.getElementById('fileNameText');
        const uploadForm = document.getElementById('uploadForm');
        const loadingOverlay = document.getElementById('loadingOverlay');
        
        // 点击选择按钮
        selectBtn.addEventListener('click', (e) => {
            e.preventDefault();
            fileInput.click();
        });
        
        // 点击上传区域
        dropZone.addEventListener('click', (e) => {
            if (e.target === dropZone || e.target.closest('.upload-icon') || e.target.closest('.upload-title') || e.target.closest('.upload-subtitle') || e.target.closest('.upload-hint')) {
                fileInput.click();
            }
        });
        
        // 文件选择变化 - 自动提交
        fileInput.addEventListener('change', (e) => {
            if (e.target.files.length > 0) {
                handleFile(e.target.files[0]);
            }
        });
        
        // 拖拽事件
        dropZone.addEventListener('dragover', (e) => {
            e.preventDefault();
            dropZone.classList.add('drag-over');
        });
        
        dropZone.addEventListener('dragleave', (e) => {
            e.preventDefault();
            dropZone.classList.remove('drag-over');
        });
        
        dropZone.addEventListener('drop', (e) => {
            e.preventDefault();
            dropZone.classList.remove('drag-over');
            
            if (e.dataTransfer.files.length > 0) {
                const file = e.dataTransfer.files[0];
                if (file.name.endsWith('.csv')) {
                    // 设置文件到input
                    const dataTransfer = new DataTransfer();
                    dataTransfer.items.add(file);
                    fileInput.files = dataTransfer.files;
                    handleFile(file);
                } else {
                    alert('请选择CSV文件');
                }
            }
        });
        
        // 处理文件 - 自动提交
        function handleFile(file) {
            fileNameText.textContent = file.name;
            fileName.classList.add('show');
            
            // 更新上传区域样式
            dropZone.style.borderColor = '#28a745';
            dropZone.style.background = 'linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%)';
            
            // 自动提交表单
            setTimeout(() => {
                loadingOverlay.classList.add('show');
                uploadForm.submit();
            }, 300);
        }
        
        // 表单提交
        uploadForm.addEventListener('submit', (e) => {
            if (fileInput.files.length === 0) {
                e.preventDefault();
                alert('请先选择文件');
                return;
            }
            loadingOverlay.classList.add('show');
        });
        
        // 触摸设备支持
        if ('ontouchstart' in window) {
            dropZone.addEventListener('touchstart', () => {
                fileInput.click();
            });
        }
    </script>
</body>
</html>`
	
	t, _ := template.New("index").Parse(tmpl)
	t.Execute(w, nil)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "文件上传失败", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 保存上传的文件
	tempFile := "/tmp/" + header.Filename
	out, err := os.Create(tempFile)
	if err != nil {
		http.Error(w, "无法创建临时文件", http.StatusInternalServerError)
		return
	}
	defer out.Close()
	io.Copy(out, file)

	// 读取并分析数据
	data, startDate, endDate, err := readCSV(tempFile)
	if err != nil {
		http.Error(w, "CSV文件解析失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 计算95th值
	result := calculateP95(data, startDate, endDate)
	log.Printf("数据解析成功: %d条数据, P95: %.2f", result.TotalSamples, result.CurrentP95)

	// 生成HTML报告
	tmpl := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Cacti®点位数据分析系统</title>
    <link rel="icon" href="/static/logo.svg" type="image/svg+xml">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.10.0/font/bootstrap-icons.css" rel="stylesheet">
    <style>
        :root {
            --primary-color: #2F5496;
            --secondary-color: #1a3a6c;
            --accent-color: #ff6b6b;
        }
        
        body {
            font-family: 'Segoe UI', 'Microsoft YaHei', sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px 0;
            zoom: 0.8;
        }
        
        .main-container {
            max-width: 1400px;
            margin: 0 auto;
        }
        
        .card {
            border: none;
            border-radius: 15px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.2);
            overflow: hidden;
        }
        
        .card-header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 50%, #f093fb 100%);
            color: white;
            padding: 25px 30px;
            border-bottom: none;
            position: relative;
            overflow: hidden;
        }
        
        .card-header::before {
            content: '';
            position: absolute;
            top: -50%;
            left: -50%;
            width: 200%;
            height: 200%;
            background: radial-gradient(circle, rgba(255,255,255,0.1) 0%, transparent 60%);
            animation: shimmer 8s ease-in-out infinite;
        }
        
        .card-header h1 {
            margin: 0;
            font-weight: 700;
            font-size: 2rem;
        }
        
        .card-body {
            padding: 30px;
        }
        
        /* P95指标卡片 */
        .p95-cards {
            display: flex;
            gap: 20px;
            margin-bottom: 30px;
        }
        
        .p95-card {
            flex: 1;
            background: linear-gradient(135deg, #fff 0%, #f8f9fa 100%);
            border-radius: 15px;
            padding: 25px;
            box-shadow: 0 5px 20px rgba(0,0,0,0.1);
            border: none;
            transition: all 0.3s ease;
            cursor: pointer;
            text-decoration: none;
            color: inherit;
            display: block;
        }
        
        .p95-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 10px 30px rgba(0,0,0,0.15);
            color: inherit;
        }
        
        .p95-card.current {
            background: #d4edda;
        }
        
        .p95-card.monthly {
            background: #fff3cd;
        }
        
        .p95-label {
            font-size: 0.9rem;
            color: #666;
            margin-bottom: 8px;
            font-weight: 500;
        }
        
        .p95-value {
            font-size: 2.5rem;
            font-weight: 700;
            color: var(--primary-color);
            margin-bottom: 5px;
        }
        
        .p95-card.current .p95-value {
            color: #28a745;
        }
        
        .p95-card.monthly .p95-value {
            color: #ffc107;
        }
        
        .p95-rank {
            font-size: 1rem;
            color: #666;
        }
        
        .p95-rank strong {
            color: #333;
        }
        
        /* 信息统计 */
        .stats-row {
            display: flex;
            gap: 15px;
            margin-bottom: 30px;
        }
        
        .stat-item {
            flex: 1;
            background: white;
            border-radius: 10px;
            padding: 15px;
            text-align: center;
            box-shadow: 0 3px 10px rgba(0,0,0,0.1);
        }
        
        .stat-icon {
            font-size: 1.5rem;
            color: var(--primary-color);
            margin-bottom: 8px;
        }
        
        .stat-value {
            font-size: 1.1rem;
            font-weight: 600;
            color: #333;
        }
        
        .stat-label {
            font-size: 0.8rem;
            color: #666;
            margin-top: 3px;
        }
        
        /* 数据表格 */
        .table-container {
            background: white;
            border-radius: 15px;
            overflow: hidden;
            box-shadow: 0 5px 20px rgba(0,0,0,0.1);
        }
        
        .table-header {
            background: linear-gradient(135deg, var(--primary-color) 0%, var(--secondary-color) 100%);
            color: white;
            padding: 15px 20px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        
        .table-header h3 {
            margin: 0;
            font-weight: 600;
        }
        
        .table-responsive {
            max-height: 600px;
            overflow-y: auto;
        }
        
        .table {
            margin: 0;
        }
        
        .table thead th {
            background: #f8f9fa;
            color: #333;
            font-weight: 600;
            border-bottom: 2px solid #dee2e6;
            padding: 12px 15px;
            position: sticky;
            top: 0;
            z-index: 10;
        }
        
        .table tbody td {
            padding: 10px 15px;
            vertical-align: middle;
            border-bottom: 1px solid #f0f0f0;
        }
        
        .table tbody tr:hover {
            background: #f8f9fa;
        }
        
        .table tbody tr.p95-row,
        .table tbody tr.p95-row > td {
            background: #d4edda !important;
            color: #155724 !important;
            font-weight: 600;
            border-color: #c3e6cb !important;
        }
        
        .table tbody tr.p95-row:hover,
        .table tbody tr.p95-row:hover > td {
            background: #c3e6cb !important;
        }
        
        .table tbody tr.monthly-p95-row,
        .table tbody tr.monthly-p95-row > td {
            background: #fff3cd !important;
            color: #856404 !important;
            font-weight: 600;
            border-color: #ffeaa7 !important;
        }
        
        .table tbody tr.monthly-p95-row:hover,
        .table tbody tr.monthly-p95-row:hover > td {
            background: #ffeaa7 !important;
        }
        
        /* 当95th和月95th是同一行时 */
        .table tbody tr.p95-row.monthly-p95-row,
        .table tbody tr.p95-row.monthly-p95-row > td {
            background: linear-gradient(90deg, #d4edda 0%, #fff3cd 100%) !important;
        }
        
        .rank-badge {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            width: 35px;
            height: 35px;
            border-radius: 50%;
            background: #e9ecef;
            color: #333;
            font-weight: 600;
            font-size: 0.85rem;
        }
        
        .rank-badge.top {
            background: linear-gradient(135deg, #ffd700 0%, #ffed4a 100%);
            color: #333;
        }
        
        .percentile-badge {
            display: inline-block;
            padding: 6px 14px;
            border-radius: 20px;
            background: linear-gradient(135deg, #6c757d 0%, #495057 100%);
            color: white;
            font-weight: 600;
            font-size: 0.95rem;
        }
        
        .traffic-bar {
            display: flex;
            align-items: center;
            gap: 10px;
        }
        
        .traffic-value {
            min-width: 80px;
            font-weight: 600;
        }
        
        .traffic-bar-bg {
 flex: 1;
            height: 8px;
            background: #e9ecef;
            border-radius: 4px;
            overflow: hidden;
        }
        
        .traffic-bar-fill {
            height: 100%;
            background: linear-gradient(90deg, var(--primary-color) 0%, #4a90d9 100%);
            border-radius: 4px;
            transition: width 0.3s ease;
        }
        
        .badge-warning {
            background: #ffc107;
            color: #333;
            font-weight: 600;
            padding: 5px 10px;
            border-radius: 15px;
            font-size: 0.8rem;
        }
        
        .badge-p95 {
            background: #ff6b6b;
            color: white;
            font-weight: 600;
            padding: 5px 10px;
            border-radius: 15px;
            font-size: 0.8rem;
        }
        
        /* 底部95th水印 */
        .watermark-bar {
            display: flex;
            justify-content: center;
            align-items: center;
            margin-top: 30px;
            padding: 20px 30px;
            background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%);
            border-radius: 15px;
            box-shadow: 0 5px 20px rgba(0,0,0,0.1);
            gap: 30px;
        }
        
        .watermark-item {
            display: flex;
            align-items: center;
            gap: 12px;
            padding: 10px 20px;
            border-radius: 12px;
            transition: all 0.3s ease;
        }
        
        .watermark-item.current {
            background: linear-gradient(135deg, rgba(40, 167, 69, 0.1) 0%, rgba(40, 167, 69, 0.05) 100%);
            border: 1px solid rgba(40, 167, 69, 0.2);
        }
        
        .watermark-item.monthly {
            background: linear-gradient(135deg, rgba(255, 193, 7, 0.1) 0%, rgba(255, 193, 7, 0.05) 100%);
            border: 1px solid rgba(255, 193, 7, 0.2);
        }
        
        .watermark-item i {
 font-size: 1.5rem;
        }
        
        .watermark-item.current i {
            color: #28a745;
        }
        
        .watermark-item.monthly i {
            color: #ffc107;
        }
        
        .watermark-label {
            font-size: 0.85rem;
            color: #666;
            font-weight: 500;
        }
        
        .watermark-value {
            font-size: 1.5rem;
            font-weight: 700;
        }
        
        .watermark-item.current .watermark-value {
            color: #28a745;
        }
        
        .watermark-item.monthly .watermark-value {
            color: #e6a800;
        }
        
        .watermark-rank {
            font-size: 0.8rem;
            color: #999;
            background: white;
            padding: 2px 8px;
            border-radius: 10px;
        }
        
        .watermark-divider {
            width: 1px;
 height: 50px;
            background: #dee2e6;
        }
            transform: translateY(-2px);
            box-shadow: 0 5px 20px rgba(47, 84, 150, 0.4);
            color: white;
        }
        
        /* 固定返回按钮 - 隐藏 */
        .btn-back-fixed {
            display: none;
        }
        
        /* 分页控件 */
        .pagination-bar {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 12px 15px;
            background: #f8f9fa;
            border-bottom: 1px solid #dee2e6;
            flex-wrap: wrap;
            gap: 10px;
        }
        
        .page-nav {
            display: flex;
            align-items: center;
            gap: 8px;
        }
        
        .page-num {
            font-weight: 500;
            color: #333;
            min-width: 120px;
            text-align: center;
        }
        
        .page-info, .page-jump {
            display: flex;
            align-items: center;
            gap: 8px;
            font-size: 0.9rem;
            color: #666;
        }
        
        .page-jump input {
            border: 1px solid #dee2e6;
            border-radius: 4px;
            padding: 2px 6px;
            text-align: center;
        }
        
        select {
            border: 1px solid #dee2e6;
            border-radius: 4px;
            padding: 4px 8px;
        }
        
        /* 响应式设计 */
        @media (max-width: 768px) {
            .p95-cards {
                flex-direction: column;
            }
            
            .stats-row {
                flex-wrap: wrap;
            }
            
            .stat-item {
                min-width: calc(50% - 10px);
            }
            
            .watermark-bar {
                flex-direction: column;
                gap: 15px;
            }
            
            .watermark-divider {
                width: 50%;
                height: 1px;
            }
        }
    </style>
</head>
<body>
    <!-- 固定返回按钮 -->
    <a href="/" class="btn-back-fixed">
        <i class="bi bi-arrow-left"></i>
        <span class="btn-back-text">返回主页</span>
    </a>
    
    <div class="main-container" style="padding-top: 50px;">
        <div class="card">
            <div class="card-header text-center">
                <h1><img src="/static/logo.svg" alt="Cacti" style="height:36px;vertical-align:middle;margin-right:10px;">Cacti®点位数据分析系统</h1>
            </div>
            <div class="card-body">
                <!-- P95指标卡片 -->
                <div class="p95-cards">
                    <a href="javascript:void(0)" onclick="goToRank({{.CurrentP95Rank}})" class="p95-card current">
                        <div class="p95-label">
                            <i class="bi bi-lightning-charge me-2"></i>当前95th值
                        </div>
                        <div class="p95-value">{{printf "%.2f" (div .CurrentP95 1000000000)}} Gbps</div>
                        <div class="p95-rank">排名: <strong>第{{.CurrentP95Rank}}名</strong> <i class="bi bi-chevron-down ms-1"></i></div>
                    </a>
                    <a href="javascript:void(0)" onclick="goToRank({{.MonthlyP95Rank}})" class="p95-card monthly">
                        <div class="p95-label">
                            <i class="bi bi-calendar-month me-2"></i>月95th值
                        </div>
                        <div class="p95-value">{{printf "%.2f" (div .MonthlyP95 1000000000)}} Gbps</div>
                        <div class="p95-rank">排名: <strong>第{{.MonthlyP95Rank}}名</strong> <i class="bi bi-chevron-down ms-1"></i></div>
                    </a>
                </div>
                
                <!-- 信息统计 -->
                <div class="stats-row">
                    <div class="stat-item">
                        <div class="stat-icon"><i class="bi bi-calendar-range"></i></div>
                        <div class="stat-value">
                            <div>起 {{.StartDate}}</div>
                            <div>止 {{.EndDate}}</div>
                        </div>
                        <div class="stat-label">数据时间范围</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-icon"><i class="bi bi-calendar3"></i></div>
                        <div class="stat-value">{{.MonthDays}}天</div>
                        <div class="stat-label">月份天数</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-icon"><i class="bi bi-database"></i></div>
                        <div class="stat-value">{{.TotalSamples}}个</div>
                        <div class="stat-label">有效数据点</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-icon"><i class="bi bi-speedometer"></i></div>
                        <div class="stat-value">{{printf "%.2f" (div .MaxTraffic 1000000000)}} Gbps</div>
                        <div class="stat-label">最大流量</div>
                    </div>
                </div>
                
                <!-- 数据表格 -->
                <div class="table-container">
                    <div class="table-header">
                        <h3><i class="bi bi-table me-2"></i>流量数据明细</h3>
                        <span class="badge bg-light text-dark">共{{.TotalSamples}}条记录</span>
                    </div>
                    
                    <!-- 分页控件 -->
                    <div class="pagination-bar">
                        <div class="page-info">
                            <span>每页显示：</span>
                            <select id="pageSize" onchange="changePageSize()">
                                <option value="50">50条</option>
                                <option value="100" selected>100条</option>
                                <option value="200">200条</option>
                                <option value="500">500条</option>
                            </select>
                        </div>
                        <div class="page-nav">
                            <button class="btn btn-sm btn-outline-primary" onclick="goToPage(1)" id="btnFirst">首页</button>
                            <button class="btn btn-sm btn-outline-primary" onclick="goToPage(currentPage-1)" id="btnPrev">上一页</button>
                            <span class="page-num" id="pageInfo">第1页/共1页</span>
                            <button class="btn btn-sm btn-outline-primary" onclick="goToPage(currentPage+1)" id="btnNext">下一页</button>
                            <button class="btn btn-sm btn-outline-primary" onclick="goToPage(totalPages)" id="btnLast">末页</button>
                        </div>
                        <div class="page-jump">
                            <span>跳转到：</span>
                            <input type="number" id="jumpPage" min="1" style="width:60px;" onkeydown="if(event.key==='Enter')jumpToPage()">
                            <button class="btn btn-sm btn-primary" onclick="jumpToPage()">GO</button>
                        </div>
                    </div>
                    
                    <div class="table-responsive">
                        <table class="table table-hover">
                            <thead>
                                <tr>
                                    <th style="width: 80px;">排名</th>
                                    <th style="width: 200px;">时间</th>
                                    <th>流量</th>
                                    <th style="width: 100px;">百分位</th>
                                </tr>
                            </thead>
                            <tbody id="tableBody">
                            </tbody>
                        </table>
                    </div>
                    
                    <!-- 底部分页 -->
                    <div class="pagination-bar">
                        <div class="page-nav">
                            <button class="btn btn-sm btn-outline-primary" onclick="goToPage(1)" id="btnFirst2">首页</button>
                            <button class="btn btn-sm btn-outline-primary" onclick="goToPage(currentPage-1)" id="btnPrev2">上一页</button>
                            <span class="page-num" id="pageInfo2">第1页/共1页</span>
                            <button class="btn btn-sm btn-outline-primary" onclick="goToPage(currentPage+1)" id="btnNext2">下一页</button>
                            <button class="btn btn-sm btn-outline-primary" onclick="goToPage(totalPages)" id="btnLast2">末页</button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
    
    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>
    <script>
        // 数据
        const allData = [
            {{range $i, $d := .SortedData}}
            { rank: {{add $i 1}}, date: "{{$d.Date}}", total: +{{printf "%.2f" (div $d.Total 1000000000)}}, percentile: +{{printf "%.1f" (percentile $i $.TotalSamples)}} }{{if not (eq (add $i 1) $.TotalSamples)}},{{end}}
            {{end}}
        ];
        
        const currentP95Rank = {{.CurrentP95Rank}};
        const monthlyP95Rank = {{.MonthlyP95Rank}};
        const maxTraffic = {{printf "%.2f" (div .MaxTraffic 1000000000)}};
        
        let currentPage = 1;
        let pageSize = 100;
        let totalPages = 1;
        
        function init() {
            totalPages = Math.ceil(allData.length / pageSize);
            renderTable();
        }
        
        function changePageSize() {
            pageSize = parseInt(document.getElementById('pageSize').value);
            currentPage = 1;
            totalPages = Math.ceil(allData.length / pageSize);
            renderTable();
        }
        
        function goToPage(page) {
            if (page < 1) page = 1;
            if (page > totalPages) page = totalPages;
            currentPage = page;
            renderTable();
        }
        
        function jumpToPage() {
            const input = document.getElementById('jumpPage');
            const page = parseInt(input.value);
            if (page >= 1 && page <= totalPages) {
                currentPage = page;
                renderTable();
            }
        }
        
        function goToRank(rank) {
            const targetPage = Math.ceil(rank / pageSize);
            if (targetPage !== currentPage) {
                currentPage = targetPage;
                renderTable();
            }
            
            setTimeout(() => {
                const rowId = rank === currentP95Rank ? 'current-p95' : 'monthly-p95';
                const row = document.getElementById(rowId);
                if (row) {
                    row.scrollIntoView({behavior: 'smooth', block: 'center'});
                }
            }, 150);
        }
        
        function renderTable() {
            const start = (currentPage - 1) * pageSize;
            const end = Math.min(start + pageSize, allData.length);
            const tbody = document.getElementById('tableBody');
            
            let html = '';
            for (let i = start; i < end; i++) {
                const d = allData[i];
                const isCurrentP95 = d.rank === currentP95Rank;
                const isMonthlyP95 = d.rank === monthlyP95Rank;
                const barWidth = maxTraffic > 0 ? (d.total / maxTraffic * 100) : 0;
                
                let rowClass = '';
                if (isCurrentP95 && isMonthlyP95) rowClass = 'p95-row monthly-p95-row';
                else if (isCurrentP95) rowClass = 'p95-row';
                else if (isMonthlyP95) rowClass = 'monthly-p95-row';
                
                let rowId = '';
                if (isCurrentP95) rowId = 'id="current-p95"';
                if (isMonthlyP95 && !isCurrentP95) rowId = 'id="monthly-p95"';
                if (isMonthlyP95 && isCurrentP95) rowId = 'id="current-p95 monthly-p95"';
                
                let badge = d.rank <= 10 ? 'top' : '';
                
                html += '<tr ' + rowId + ' class="' + rowClass + '">';
                html += '<td><span class="rank-badge ' + badge + '">' + d.rank + '</span></td>';
                html += '<td><i class="bi bi-clock me-2"></i>' + d.date + '</td>';
                html += '<td><div class="traffic-bar"><span class="traffic-value">' + d.total.toFixed(2) + ' Gbps</span><div class="traffic-bar-bg"><div class="traffic-bar-fill" style="width:' + barWidth.toFixed(0) + '%"></div></div></div></td>';
                html += '<td><span class="percentile-badge">' + d.percentile + '%</span></td>';
                html += '</tr>';
            }
            
            tbody.innerHTML = html;
            
            // 更新分页信息
            document.getElementById('pageInfo').textContent = '第' + currentPage + '页/共' + totalPages + '页';
            document.getElementById('pageInfo2').textContent = '第' + currentPage + '页/共' + totalPages + '页';
            
            // 更新按钮状态
            document.getElementById('btnFirst').disabled = currentPage === 1;
            document.getElementById('btnPrev').disabled = currentPage === 1;
            document.getElementById('btnNext').disabled = currentPage === totalPages;
            document.getElementById('btnLast').disabled = currentPage === totalPages;
            document.getElementById('btnFirst2').disabled = currentPage === 1;
            document.getElementById('btnPrev2').disabled = currentPage === 1;
            document.getElementById('btnNext2').disabled = currentPage === totalPages;
            document.getElementById('btnLast2').disabled = currentPage === totalPages;
            
            // 滚动到锚点
            const hash = window.location.hash;
            if (hash) {
                const el = document.querySelector(hash);
                if (el) {
                    setTimeout(() => el.scrollIntoView({behavior: 'smooth', block: 'center'}), 100);
                }
            }
        }
        
        // 双击背景区域返回主页
        document.addEventListener('dblclick', function(e) {
            // 检查是否点击在卡片外部（背景区域）
            const card = document.querySelector('.card');
            if (card && !card.contains(e.target)) {
                window.location.href = '/';
            }
        });
        
        // 初始化
        init();
    </script>
</body>
</html>`

	funcMap := template.FuncMap{
		"div": func(a, b float64) float64 {
			return a / b
		},
		"mul": func(a, b float64) float64 {
			return a * b
		},
		"add": func(a, b int) int {
			return a + b
		},
		"percentile": func(index, total int) float64 {
			return float64(total-index) / float64(total) * 100
		},
	}

	t, err := template.New("result").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		log.Printf("模板解析错误: %v", err)
		http.Error(w, "模板解析错误", http.StatusInternalServerError)
		return
	}

	if err := t.Execute(w, result); err != nil {
		log.Printf("模板执行错误: %v, TotalSamples: %d, SortedData len: %d", err, result.TotalSamples, len(result.SortedData))
	}

	// 清理临时文件
	os.Remove(tempFile)
}

func main() {
	// 静态文件
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/upload", uploadHandler)

	port := ":8888"
	fmt.Printf("CactiP95 已启动，访问 http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
