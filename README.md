# SSH-Manager

Linux 系统的 SSH 连接管理 CLI 工具，支持查看连接、断开会话、禁止/解除禁止用户或 IP、实时监控面板、SSH 隧道端口转发检测。

## 功能特性

- **会话列表** - 列出所有活跃的 SSH 连接
- **端口转发检测** - 显示 SSH 隧道的端口转发信息（本地 `-L`、远程 `-R`、动态 `-D`）
- **断开连接** - 支持按 PID、用户名、IP、TTY 断开连接（需二次确认）
- **禁止连接** - 禁止指定用户或 IP 连接 SSH
- **解除禁止** - 移除禁止规则
- **重载规则** - 重新应用禁止规则
- **监控面板** - 交互式终端 UI，实时监控 SSH 连接

## 环境准备

### 系统要求

- Linux 系统
- Go 1.22+

### 安装 Go

```bash
# 下载 Go
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz

# 解压到 /usr/local
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz

# 配置环境变量
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 验证安装
go version
```

### 依赖工具

确保以下工具已安装（大多数 Linux 系统默认已包含）：

```bash
which ss ps who
```

## 编译打包

### 获取源码

```bash
git clone <repository-url>
cd ssh-manager
```

### 编译

```bash
# 直接编译
go build -o ssh-manager .

# 编译并指定输出路径
go build -o bin/ssh-manager .
```

### 安装到系统

```bash
# 安装到 /usr/local/bin（需要 root 权限）
sudo mv ssh-manager /usr/local/bin/
sudo chmod +x /usr/local/bin/ssh-manager

# 或安装到用户目录
mkdir -p ~/.local/bin
mv ssh-manager ~/.local/bin/
echo 'export PATH=$PATH:~/.local/bin' >> ~/.bashrc
```

### 交叉编译

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o ssh-manager-linux-amd64 .

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o ssh-manager-linux-arm64 .

# macOS AMD64
GOOS=darwin GOARCH=amd64 go build -o ssh-manager-darwin-amd64 .

# macOS ARM64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o ssh-manager-darwin-arm64 .
```

## 使用方法

### 列出 SSH 连接

```bash
# 表格格式（默认）
ssh-manager list

# JSON 格式输出
ssh-manager list -o json

# 仅显示空闲超过 30 分钟的会话
ssh-manager list -i 30

# 仅显示 root 用户的会话
ssh-manager list -u root

# 使用交互式 TUI
ssh-manager list --tui
```

### 输出示例

```
PID    USER       TTY      IP              LOGIN TIME           IDLE
----------------------------------------------------------------------
398001 root       pts/0    125.35.5.249    2026-04-17 12:43     01h46m
398001 root       pts/1    125.35.5.249    2026-04-17 14:29     <1m
----------------------------------------------------------------------
SSH Tunnels (no TTY):
447352 ano_sub+   -
         \__ L:* -> 127.0.0.1:19876
447578 root       -
         \__ L:* -> 127.0.0.1:9999
----------------------------------------------------------------------
Total: 2 sessions, 2 tunnels
```

#### 端口转发类型说明

| 类型 | 说明 | 示例 |
|------|------|------|
| `L` | 本地端口转发 `-L` | `L:* -> 192.168.1.1:80` |
| `R` | 远程端口转发 `-R` | `R:0.0.0.0:8080 -> *` |
| `D` | 动态 SOCKS 代理 `-D` | `D:125.35.5.249:53132 (SOCKS)` |

说明：
- `L:* -> host:port` 表示本地端口转发到远程主机的指定端口
- 同一会话的多个相同转发目标会合并显示
- 纯隧道（无 TTY 登录）会单独显示在 "SSH Tunnels" 部分

#### JSON 输出格式

```json
{
  "sessions": [
    {
      "pid": 398001,
      "user": "root",
      "tty": "pts/0",
      "ip": "125.35.5.249",
      "login_time": "2026-04-17T12:43:00+08:00",
      "idle_time": "1h46m21s"
    }
  ],
  "tunnels": [
    {
      "pid": 447352,
      "user": "ano_sub+",
      "ip": "",
      "forwards": ["L:* -> 127.0.0.1:19876"]
    }
  ]
}
```

### 断开连接

```bash
# 按 PID 断开（会提示确认）
ssh-manager kill 12345

# 强制断开（跳过确认）
ssh-manager kill 12345 -f

# 按用户名断开所有会话
ssh-manager kill user:root

# 按 IP 断开所有会话
ssh-manager kill ip:192.168.1.100

# 按 TTY 断开会话
ssh-manager kill tty:pts/0
```

### 禁止连接

禁止规则**直接生效**，会同时更新 `hosts.deny` 和 `iptables`。

```bash
# 禁止指定用户
ssh-manager ban --user hacker

# 禁止指定 IP
ssh-manager ban --ip 1.2.3.4

# 禁止 IP 段
ssh-manager ban --range 192.168.0.0/24

# 禁止所有当前连接的用户
ssh-manager ban --all-users

# 禁止所有当前连接的 IP
ssh-manager ban --all-ips

# 组合禁止
ssh-manager ban --user baduser --ip 1.2.3.4 --range 10.0.0.0/8
```

### 解除禁止

解除规则同样**直接生效**。

```bash
# 解除指定用户
ssh-manager unban --user hacker

# 解除指定 IP
ssh-manager unban --ip 1.2.3.4

# 解除 IP 段
ssh-manager unban --range 192.168.0.0/24

# 解除所有禁止
ssh-manager unban --all
```

### 重载规则

从配置文件重新加载并应用禁止规则。

适用场景：
- 系统重启后恢复禁止规则
- 直接修改配置文件后重新应用
- 程序异常退出后恢复规则

```bash
ssh-manager reload
```

### 监控面板

```bash
ssh-manager monitor
```

交互式界面，支持以下操作：
- `q` - 退出
- `r` - 刷新
- `k` - 断开选中的会话
- `b` - 禁止选中会话的 IP

## 配置文件

配置文件位置（按优先级）：
1. `~/.ssh-manager/config.yaml`（用户级）
2. `/etc/ssh-manager/config.yaml`（系统级）

### 配置示例

```yaml
general:
  # 检查间隔（秒）
  check_interval: 5

  # 禁止规则保存路径
  config_path: /etc/ssh-manager/ban_rules.yaml

ban:
  # 启用禁止功能
  enabled: true

  # 默认禁止的用户列表
  users: []

  # 默认禁止的 IP 列表
  ips: []

  # 默认禁止的 IP 段
  ip_ranges: []

  # 使用 /etc/hosts.deny
  use_hosts_deny: true

  # 使用 iptables
  use_iptables: true

monitor:
  # 自动断开空闲会话
  auto_kill_idle: false

  # 空闲阈值（分钟）
  idle_threshold: 30

  # 刷新间隔（秒）
  refresh_seconds: 5
```

## 工作原理

### 会话扫描

工具通过组合多个系统命令获取 SSH 会话信息：
1. `who -u` - 获取登录用户、TTY、来源 IP，空闲时间
2. `ps aux` - 获取 sshd 进程 PID 和 TTY 映射
3. `ss -tnp` - 获取网络连接状态和端口转发信息

### 端口转发检测

通过解析 `ss -tnp` 输出中 sshd 进程的网络连接来识别端口转发：
- 监听本地非 22 端口的 sshd 连接 → 本地/动态转发
- 分析连接对端地址判断转发类型
- 同一用户的相同转发目标会合并显示

### 禁止机制

支持两种禁止方式：

1. **TCP Wrappers** (`/etc/hosts.deny`)
   - 自动修改 `/etc/hosts.deny` 文件
   - 添加 `sshd: <ip>` 规则

2. **iptables**
   - 创建专用 chain `SSH-MANAGER-BAN`
   - 添加 REJECT 规则

## 权限要求

- 读取 SSH 连接信息：普通用户
- 断开连接：普通用户
- 禁止/解除禁止：需要 root 权限
- 监控面板：普通用户

## 常见问题

### Q: list 命令显示 "No active SSH sessions found"

确保有 SSH 连接存在，可以使用 `who` 命令确认。

### Q: 禁止规则不生效

1. 确认以 root 权限运行
2. 使用 `ssh-manager reload` 重新加载规则
3. 检查 `/etc/hosts.deny` 是否已更新

### Q: monitor 命令报错 "open /dev/tty: no such device"

monitor 需要交互式终端环境，无法在非交互式 shell 中运行。

## 注意事项

1. 断开连接会立即终止对应 SSH 会话
2. 禁止规则修改需要 root 权限
3. 建议先测试 `list` 命令确认能正常获取会话信息
4. 使用 `reload` 命令确保禁止规则生效
5. SSH 隧道（无 TTY）会单独显示在 "SSH Tunnels" 区域
6. 端口转发按目标端口去重，同一目标只显示一次

## 项目结构

```
ssh-manager/
├── cmd/
│   ├── root.go          # 根命令
│   ├── list.go          # list - 列出连接
│   ├── kill.go          # kill - 断开连接
│   ├── ban.go           # ban - 禁止连接
│   ├── unban.go         # unban - 解除禁止
│   ├── reload.go        # reload - 重载规则
│   ├── monitor.go       # monitor - 监控面板
│   └── version.go       # version - 版本
├── internal/
│   ├── config.go       # 配置管理
│   ├── session.go      # 会话数据结构
│   ├── scanner.go      # 会话扫描逻辑
│   ├── killer.go       # 终止连接逻辑
│   ├── banner.go       # 禁止规则管理
│   ├── iptables.go    # iptables 操作
│   └── tui.go         # 终端 UI
├── config.example.yaml  # 配置示例
├── main.go
├── go.mod
└── README.md
```

## 许可证

MIT License

## 版本

v1.0.0
