# Session存储方案设计

## 混合存储架构

### SQLite存储的内容
```sql
-- Session元数据表
CREATE TABLE sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    connection_name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP,
    status TEXT DEFAULT 'active' -- active, closed, archived
);

-- 查询历史表
CREATE TABLE query_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER REFERENCES sessions(id),
    query TEXT NOT NULL,
    executed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    execution_time_ms INTEGER,
    rows_affected INTEGER,
    success BOOLEAN,
    error_message TEXT,
    result_file_path TEXT -- 指向文件系统中的结果文件
);

-- 用户设置表
CREATE TABLE user_preferences (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Session统计表
CREATE TABLE session_stats (
    session_id INTEGER PRIMARY KEY REFERENCES sessions(id),
    total_queries INTEGER DEFAULT 0,
    successful_queries INTEGER DEFAULT 0,
    total_execution_time_ms INTEGER DEFAULT 0,
    last_query_at DATETIME
);
```

### 文件系统存储的内容
```
~/.config/sqlterm/
├── sessions.db                 # SQLite数据库
└── session_data/
    ├── {connection_name}/
    │   ├── results/
    │   │   ├── query_001_result.csv
    │   │   ├── query_002_result.md
    │   │   └── query_003_export.json
    │   ├── exports/
    │   │   └── data_export_20240101.csv
    │   └── temp/
    │       └── temp_query_result.tmp
    └── global/
        ├── ai_context.json      # AI对话上下文
        └── cache/
```

## 实现优势

1. **最佳性能** - 元数据查询使用SQL，大文件直接访问文件系统
2. **数据完整性** - 重要元数据有事务保护
3. **可维护性** - 结构化数据易于管理，文件数据易于调试
4. **扩展性** - 可以轻松添加新的查询和统计功能
5. **备份策略** - 分层备份：数据库用于关键元数据，文件用于结果数据

## 迁移策略

1. **渐进式迁移** - 保持文件接口不变，底层逐步切换到SQLite
2. **向后兼容** - 支持读取现有文件格式数据
3. **数据导入** - 提供工具将现有文件数据导入到SQLite

## API设计示例

```go
type SessionManager interface {
    // 元数据操作（使用SQLite）
    CreateSession(connectionName string) (*Session, error)
    GetSessionHistory(connectionName string, limit int) ([]*QueryRecord, error)
    RecordQuery(sessionID int64, query string, result *QueryResult) error
    GetSessionStats(sessionID int64) (*SessionStats, error)
    
    // 文件操作（使用文件系统）
    SaveQueryResult(sessionID int64, result *QueryResult, format string) (string, error)
    LoadQueryResult(filePath string) (*QueryResult, error)
    ExportData(sessionID int64, format string, options ExportOptions) (string, error)
    
    // 清理操作
    CleanupOldSessions(retentionDays int) error
    CleanupResultFiles(retentionDays int) error
}
```

这种混合方案可以发挥两种存储方式的优势，避免各自的缺点，为sqlterm提供高性能和高可维护性的session管理。