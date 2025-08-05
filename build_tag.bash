#!/bin/bash

# 检查是否提供了TAG_NAME参数
if [ -z "$1" ]; then
    echo "错误: TAG_NAME是必传参数"
    echo "用法: $0 <TAG_NAME>"
    exit 1
fi

TAG_NAME=$1

# 检查是否有未提交的更改
if [[ -n $(git status --porcelain) ]]; then
    echo "警告：存在未提交的更改，请先提交或暂存更改"
    git status --porcelain
    exit 1
fi

# 创建标签
echo "正在创建标签 $TAG_NAME..."
git tag $TAG_NAME

# 检查标签是否创建成功
if [ $? -eq 0 ]; then
    echo "标签 $TAG_NAME 创建成功"
else
    echo "标签创建失败"
    exit 1
fi

# 推送标签到远程仓库
echo "正在推送标签 $TAG_NAME 到远程仓库..."
git push origin $TAG_NAME

# 检查推送是否成功
if [ $? -eq 0 ]; then
    echo "标签 $TAG_NAME 推送成功"
else
    echo "标签推送失败"
    exit 1
fi

echo "标签 $TAG_NAME 已成功创建并推送到远程仓库"
