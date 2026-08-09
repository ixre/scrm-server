-- 中文全文检索：安装 zhparser 扩展并创建全文索引。
-- 替代 MySQL 的 FULLTEXT + ngram，用于 chat_msg.content_text 的中文搜索。
--
-- 前提：PG 镜像需预装 zhparser（见 docs/postgres-zhparser.Dockerfile）。
-- 幂等：本文件由 internal/migrate 在事务内执行一次，以文件名为版本号记录进
-- schema_migrations，已记录的文件重复运行不会再次执行。

-- 1. 安装 zhparser 扩展（需超级用户；docker-compose 的 postgres 用户即超级用户）
CREATE EXTENSION IF NOT EXISTS zhparser;

-- 2. 基于 zhparser 创建中文全文搜索配置
--    注：PG 的 CREATE TEXT SEARCH CONFIGURATION 无 IF NOT EXISTS 语法，
--    但本迁移文件只执行一次（由 schema_migrations 保证），无需额外幂等处理。
CREATE TEXT SEARCH CONFIGURATION chinese (PARSER = zhparser);

-- 3. 将名词(n)、动词(v)、形容词(a)等映射到 simple 词典（不做词干提取，保留原词）
ALTER TEXT SEARCH CONFIGURATION chinese ADD MAPPING FOR n,v,a,i,e,l WITH simple;

-- 4. 在 chat_msg.content_text 上创建 GIN 全文索引（使用 chinese 配置）
CREATE INDEX IF NOT EXISTS idx_chat_msg_content_text_zh
    ON chat_msg USING gin (to_tsvector('chinese', content_text));
