#!/bin/bash

# 日志轮转脚本
# 用于管理和轮转日志文件

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# 配置
LOG_DIR=${LOG_DIR:-"logs"}
MAX_SIZE=${MAX_SIZE:-100M}
KEEP_DAYS=${KEEP_DAYS:-30}
COMPRESS=${COMPRESS:-true}

show_help() {
    echo "V Panel 日志轮转脚本"
    echo ""
    echo "用法:"
    echo "  $0 rotate             轮转日志文件"
    echo "  $0 clean              清理旧日志"
    echo "  $0 analyze            分析日志"
    echo "  $0 setup              设置自动轮转"
    echo ""
    echo "环境变量:"
    echo "  LOG_DIR               日志目录 (默认: logs)"
    echo "  MAX_SIZE              最大文件大小 (默认: 100M)"
    echo "  KEEP_DAYS             保留天数 (默认: 30)"
    echo "  COMPRESS              是否压缩 (默认: true)"
    echo ""
}

rotate_logs() {
    echo -e "${YELLOW}轮转日志文件...${NC}"
    
    if [ ! -d "$LOG_DIR" ]; then
        echo -e "${YELLOW}⚠ 日志目录不存在: $LOG_DIR${NC}"
        return 0
    fi
    
    timestamp=$(date +%Y%m%d_%H%M%S)
    rotated_count=0
    
    # 查找需要轮转的日志文件
    for log_file in "$LOG_DIR"/*.log; do
        if [ ! -f "$log_file" ]; then
            continue
        fi
        
        # 检查文件大小
        file_size=$(stat -f%z "$log_file" 2>/dev/null || stat -c%s "$log_file" 2>/dev/null || echo "0")
        max_size_bytes=$(echo "$MAX_SIZE" | sed 's/M/*1024*1024/;s/K/*1024/;s/G/*1024*1024*1024/' | bc)
        
        if [ "$file_size" -gt "$max_size_bytes" ]; then
            base_name=$(basename "$log_file" .log)
            rotated_file="$LOG_DIR/${base_name}_${timestamp}.log"
            
            echo "轮转: $log_file -> $rotated_file"
            
            # 复制并清空原文件
            cp "$log_file" "$rotated_file"
            > "$log_file"
            
            # 压缩
            if [ "$COMPRESS" = "true" ]; then
                gzip "$rotated_file"
                echo "  已压缩: ${rotated_file}.gz"
            fi
            
            ((rotated_count++))
        fi
    done
    
    if [ $rotated_count -eq 0 ]; then
        echo -e "${GREEN}✓ 没有需要轮转的日志${NC}"
    else
        echo -e "${GREEN}✓ 已轮转 $rotated_count 个日志文件${NC}"
    fi
}

clean_old_logs() {
    echo -e "${YELLOW}清理旧日志...${NC}"
    echo "保留最近 $KEEP_DAYS 天的日志"
    
    if [ ! -d "$LOG_DIR" ]; then
        echo -e "${YELLOW}⚠ 日志目录不存在: $LOG_DIR${NC}"
        return 0
    fi
    
    deleted_count=0
    deleted_size=0
    
    # 查找并删除旧日志
    while IFS= read -r -d '' file; do
        file_size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null || echo "0")
        deleted_size=$((deleted_size + file_size))
        
        echo "删除: $file"
        rm -f "$file"
        ((deleted_count++))
    done < <(find "$LOG_DIR" -type f \( -name "*.log.*" -o -name "*.log.gz" \) -mtime +$KEEP_DAYS -print0)
    
    if [ $deleted_count -eq 0 ]; then
        echo -e "${GREEN}✓ 没有需要清理的旧日志${NC}"
    else
        deleted_size_mb=$((deleted_size / 1024 / 1024))
        echo -e "${GREEN}✓ 已删除 $deleted_count 个文件，释放 ${deleted_size_mb}MB 空间${NC}"
    fi
}

analyze_logs() {
    echo -e "${YELLOW}分析日志...${NC}"
    echo ""
    
    if [ ! -d "$LOG_DIR" ]; then
        echo -e "${YELLOW}⚠ 日志目录不存在: $LOG_DIR${NC}"
        return 0
    fi
    
    # 统计日志文件
    echo "日志文件统计:"
    total_size=0
    file_count=0
    
    for log_file in "$LOG_DIR"/*; do
        if [ -f "$log_file" ]; then
            file_size=$(stat -f%z "$log_file" 2>/dev/null || stat -c%s "$log_file" 2>/dev/null || echo "0")
            total_size=$((total_size + file_size))
            ((file_count++))
        fi
    done
    
    total_size_mb=$((total_size / 1024 / 1024))
    echo "  文件数: $file_count"
    echo "  总大小: ${total_size_mb}MB"
    echo ""
    
    # 分析错误日志
    echo "错误统计 (最近 1000 行):"
    for log_file in "$LOG_DIR"/*.log; do
        if [ ! -f "$log_file" ]; then
            continue
        fi
        
        error_count=$(tail -1000 "$log_file" 2>/dev/null | grep -c "ERROR" || echo "0")
        warn_count=$(tail -1000 "$log_file" 2>/dev/null | grep -c "WARN" || echo "0")
        
        if [ "$error_count" -gt 0 ] || [ "$warn_count" -gt 0 ]; then
            echo "  $(basename "$log_file"):"
            echo "    ERROR: $error_count"
            echo "    WARN: $warn_count"
        fi
    done
    echo ""
    
    # 最近的错误
    echo "最近的错误 (最多 5 条):"
    for log_file in "$LOG_DIR"/*.log; do
        if [ ! -f "$log_file" ]; then
            continue
        fi
        
        tail -1000 "$log_file" 2>/dev/null | grep "ERROR" | tail -5 | while read -r line; do
            echo "  $line"
        done
    done
}

setup_logrotate() {
    echo -e "${YELLOW}设置自动日志轮转...${NC}"
    
    # 创建 logrotate 配置
    config_file="/etc/logrotate.d/vpanel"
    
    echo "创建配置文件: $config_file"
    
    sudo tee "$config_file" > /dev/null <<EOF
# V Panel 日志轮转配置

$(pwd)/$LOG_DIR/*.log {
    daily
    rotate 30
    missingok
    notifempty
    compress
    delaycompress
    copytruncate
    maxsize $MAX_SIZE
    dateext
    dateformat -%Y%m%d
}
EOF
    
    echo -e "${GREEN}✓ logrotate 配置已创建${NC}"
    echo ""
    echo "测试配置:"
    sudo logrotate -d "$config_file"
    echo ""
    echo "手动运行轮转:"
    echo "  sudo logrotate -f $config_file"
}

setup_cron() {
    echo -e "${YELLOW}设置定时任务...${NC}"
    
    script_path=$(realpath "$0")
    
    # 检查是否已存在
    if crontab -l 2>/dev/null | grep -q "$script_path"; then
        echo -e "${YELLOW}⚠ 定时任务已存在${NC}"
        return 0
    fi
    
    # 添加定时任务
    (crontab -l 2>/dev/null; echo "0 2 * * * $script_path rotate && $script_path clean") | crontab -
    
    echo -e "${GREEN}✓ 定时任务已添加${NC}"
    echo "每天凌晨 2 点自动轮转和清理日志"
    echo ""
    echo "查看定时任务:"
    echo "  crontab -l"
}

show_status() {
    echo -e "${YELLOW}日志状态:${NC}"
    echo ""
    
    if [ ! -d "$LOG_DIR" ]; then
        echo -e "${YELLOW}⚠ 日志目录不存在: $LOG_DIR${NC}"
        return 0
    fi
    
    # 列出日志文件
    echo "当前日志文件:"
    ls -lh "$LOG_DIR" | tail -n +2 | while read -r line; do
        echo "  $line"
    done
    echo ""
    
    # 检查 logrotate 配置
    if [ -f "/etc/logrotate.d/vpanel" ]; then
        echo -e "${GREEN}✓ logrotate 配置已设置${NC}"
    else
        echo -e "${YELLOW}⚠ logrotate 配置未设置${NC}"
        echo "  运行: $0 setup"
    fi
    
    # 检查定时任务
    if crontab -l 2>/dev/null | grep -q "log-rotate.sh"; then
        echo -e "${GREEN}✓ 定时任务已设置${NC}"
    else
        echo -e "${YELLOW}⚠ 定时任务未设置${NC}"
        echo "  运行: $0 setup"
    fi
}

# 主逻辑
case "$1" in
    rotate)
        rotate_logs
        ;;
    clean)
        clean_old_logs
        ;;
    analyze)
        analyze_logs
        ;;
    setup)
        setup_logrotate
        echo ""
        setup_cron
        ;;
    status)
        show_status
        ;;
    *)
        show_help
        exit 1
        ;;
esac

echo ""
echo -e "${GREEN}完成！${NC}"
