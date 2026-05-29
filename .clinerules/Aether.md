使用 PostgreSQL 16 + pgvector 实现向量检索，不要引入其他向量数据库。

所有敏感配置（如数据库密码）必须通过环境变量注入，严禁硬编码。

混合检索的融合公式 final_score = α * BM25_norm + (1-α) * (1 - cosine_distance)，其中 α 默认值为 0.5。