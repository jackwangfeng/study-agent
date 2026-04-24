# study-agent

高二化学学习代理：拍题 → AI 解析 + 提示阶梯 → 自动错题本 + 间隔重复变式重考。

## Stack

- **后端**：Go (gin + gorm)，部署阿里云轻量应用服务器
- **数据库**：PostgreSQL（同机 docker）
- **对象存储**：阿里云 OSS（化学题图片）
- **LLM**：DashScope Qwen2.5-VL-Max（vision + 推理一体），后续可拆 OCR/推理两步
- **前端**：Flutter PWA，iPad 加书签到主屏

后端框架 fork 自姊妹项目 [recompdaily](https://github.com/jackwangfeng/loss-weight)。
