package core

// 这个文件提供了 Credential 的使用示例，仅用于文档说明，不会被编译

/*
使用示例1：在 MessageRunner 中获取邮件配置

```go
// worker/pkg/runner/message.go

type MessageRunner struct {
    task      *core.Task
    config    MessageConfig
    apiserver core.Apiserver  // 依赖注入
}

func (r *MessageRunner) Execute(ctx context.Context, logChan chan<- string) (*core.Result, error) {
    // 1. 获取凭证
    cred, err := r.apiserver.GetCredential(r.config.CredentialID)
    if err != nil {
        return nil, fmt.Errorf("获取凭证失败: %w", err)
    }

    logChan <- fmt.Sprintf("成功获取凭证: %s (类型: %s)", cred.Name, cred.Category)

    // 2. 根据凭证类型使用不同的字段
    switch r.config.Type {
    case "email":
        return r.sendEmail(cred, logChan)
    case "dingtalk":
        return r.sendDingtalk(cred, logChan)
    default:
        return nil, fmt.Errorf("不支持的消息类型: %s", r.config.Type)
    }
}

// 发送邮件示例
func (r *MessageRunner) sendEmail(cred *core.Credential, logChan chan<- string) (*core.Result, error) {
    // 方法1：使用 MustGetXxx 获取必填字段（不存在会panic）
    smtpHost := cred.MustGetString("smtp_host")
    smtpPort := cred.MustGetInt("smtp_port")
    username := cred.MustGetString("username")
    password := cred.MustGetString("password")

    // 方法2：使用 GetXxx 获取可选字段（带默认值）
    fromName, ok := cred.GetString("from_name")
    if !ok {
        fromName = "系统通知"  // 使用默认值
    }

    useTLS, ok := cred.GetBool("use_tls")
    if !ok {
        useTLS = true  // 使用默认值
    }

    logChan <- fmt.Sprintf("连接SMTP服务器: %s:%d (TLS: %v)", smtpHost, smtpPort, useTLS)

    // 3. 使用凭证信息发送邮件
    // ... 实际发送邮件的代码

    return &core.Result{
        Status: core.StatusSuccess,
        Output: "邮件发送成功",
    }, nil
}

// 发送钉钉通知示例
func (r *MessageRunner) sendDingtalk(cred *core.Credential, logChan chan<- string) (*core.Result, error) {
    // 获取钉钉webhook
    webhook := cred.MustGetString("webhook")

    logChan <- fmt.Sprintf("发送钉钉消息到: %s", webhook)

    // 构建钉钉消息
    message := map[string]interface{}{
        "msgtype": "text",
        "text": map[string]interface{}{
            "content": r.config.Content,
        },
    }

    // 发送HTTP请求到钉钉webhook
    // ... 实际发送的代码

    return &core.Result{
        Status: core.StatusSuccess,
        Output: "钉钉消息发送成功",
    }, nil
}
```

---

使用示例2：在 DatabaseRunner 中获取数据库连接信息

```go
// worker/pkg/runner/database.go

type DatabaseRunner struct {
    task      *core.Task
    config    DatabaseConfig
    apiserver core.Apiserver
}

func (r *DatabaseRunner) Execute(ctx context.Context, logChan chan<- string) (*core.Result, error) {
    // 1. 获取数据库凭证
    cred, err := r.apiserver.GetCredential(r.config.CredentialID)
    if err != nil {
        return nil, fmt.Errorf("获取数据库凭证失败: %w", err)
    }

    // 2. 构建数据库连接字符串
    host := cred.MustGetString("host")
    port := cred.MustGetInt("port")
    database := cred.MustGetString("database")
    username := cred.MustGetString("username")
    password := cred.MustGetString("password")

    dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
        username, password, host, port, database)

    logChan <- fmt.Sprintf("连接数据库: %s:%d/%s", host, port, database)

    // 3. 连接数据库并执行查询
    // ... 实际数据库操作

    return &core.Result{
        Status: core.StatusSuccess,
        Output: "数据库操作成功",
    }, nil
}
```

---

使用示例3：错误处理

```go
func (r *MessageRunner) Execute(ctx context.Context, logChan chan<- string) (*core.Result, error) {
    // 获取凭证并处理错误
    cred, err := r.apiserver.GetCredential(r.config.CredentialID)
    if err != nil {
        logChan <- fmt.Sprintf("❌ 获取凭证失败: %v", err)
        return &core.Result{
            Status: core.StatusError,
            Output: fmt.Sprintf("获取凭证失败: %v", err),
        }, err
    }

    // 验证凭证类型是否匹配
    expectedCategory := "email"
    if cred.Category != expectedCategory {
        err := fmt.Errorf("凭证类型不匹配：期望 %s，实际 %s", expectedCategory, cred.Category)
        logChan <- fmt.Sprintf("❌ %v", err)
        return &core.Result{
            Status: core.StatusError,
            Output: err.Error(),
        }, err
    }

    // 安全地获取字段（避免panic）
    username, ok := cred.GetString("username")
    if !ok {
        err := fmt.Errorf("凭证缺少必填字段: username")
        logChan <- fmt.Sprintf("❌ %v", err)
        return &core.Result{
            Status: core.StatusError,
            Output: err.Error(),
        }, err
    }

    // ... 后续逻辑
}
```

---

凭证类型字段说明：

1. email（邮件配置）
   - smtp_host: string  (必填) SMTP服务器地址
   - smtp_port: int     (必填) SMTP端口号
   - username: string   (必填) 邮箱账号
   - password: string   (必填) 邮箱密码
   - from_name: string  (可选) 发件人名称
   - use_tls: bool      (可选) 是否使用TLS

2. username_password（用户名+密码）
   - username: string   (必填) 用户名
   - password: string   (必填) 密码

3. api_token（API令牌）
   - name: string       (可选) Token名称
   - token: string      (必填) Token值
   - expires_at: string (可选) 过期时间

4. ssh_private_key（SSH私钥）
   - username: string      (可选) SSH用户名
   - private_key: string   (必填) SSH私钥内容
   - passphrase: string    (可选) 私钥密码

5. web_auth（Web认证）
   - url: string        (必填) 登录URL
   - username: string   (必填) 用户名
   - password: string   (必填) 密码

6. secret_text（秘密文本）
   - secret: string     (必填) 秘密文本内容

---

注意事项：

1. 凭证获取会调用 API Server 的解密接口，返回的是明文数据
2. 凭证数据仅在内存中使用，不要持久化到磁盘或日志
3. 如果凭证已被禁用（is_active=false），会返回错误
4. 使用 MustGetXxx 方法会在字段不存在时panic，确保在使用前已验证
5. 使用 GetXxx 方法更安全，可以处理字段不存在的情况
6. 凭证获取失败时，应该记录日志并返回友好的错误信息

*/
