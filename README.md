# Emby 终端播放器

一个基于 TUI 的 Emby 媒体服务器客户端，支持在终端中浏览和播放 Emby 媒体库。

## 功能特性

- 📺 媒体库浏览
- 🔐 Token 自动缓存与刷新
- 🎬 基于 mpv 的视频播放
- 🎨 简洁的 TUI 界面

## 快速开始

```bash
export EMBY_SERVER="http://your-server:8096"
export EMBY_USERNAME="your-username"
export EMBY_PASSWORD="your-password"
go run main.go
```

## 环境变量

| 变量 | 说明 |
|------|------|
| EMBY_SERVER | Emby 服务器地址 |
| EMBY_USERNAME | 用户名 |
| EMBY_PASSWORD | 密码 |

## 依赖

- [mpv](https://mpv.io/) - 视频播放器

## 预览

![界面1](image.png)

![界面2](image2.png)
