# 基于 PostgreSQL 16 编译安装 zhparser 中文分词扩展。
# 用于 chat_msg 表的中文全文检索（替代 MySQL FULLTEXT + ngram）。
#
# 构建：
#   docker build -t scrm-pg-zhparser:16 -f docs/postgres-zhparser.Dockerfile docs/
FROM postgres:16

# zhparser 依赖 pg_config（在 postgresql-server-dev）与 git/make
RUN apt-get update && apt-get install -y --no-install-recommends \
        postgresql-server-dev-16 \
        git \
        make \
        gcc \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 编译安装 zhparser
RUN git clone --depth 1 https://github.com/amutu/zhparser.git /tmp/zhparser \
    && cd /tmp/zhparser \
    && make && make install \
    && rm -rf /tmp/zhparser

# 清理编译工具，保持镜像精简
RUN apt-get purge -y postgresql-server-dev-16 git make gcc ca-certificates \
    && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/*
