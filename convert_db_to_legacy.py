"""
convert_db_to_legacy.py

将新版本 (tmd) 的 SQLite 数据库转换为旧版本 (tmd-2.4.4) 兼容格式。

Schema 差异对照:
  - users:           新版多出 is_accessible 列 → 旧版无此列，丢弃
  - user_previous_names: 新版列名 user_id      → 旧版列名 uid
  - lsts:             新版列名 owner_user_id   → 旧版列名 owner_uid
  - lst_entities / user_entities / user_links: 结构一致，直接复制

用法:
  python convert_db_to_legacy.py <新版db路径> <输出旧版db路径>

示例:
  python convert_db_to_legacy.py new_tmd.db legacy_tmd.db
"""

import sqlite3
import sys
import os


def convert(new_db_path: str, legacy_db_path: str):
    if not os.path.isfile(new_db_path):
        print(f"[ERROR] 新版数据库文件不存在: {new_db_path}")
        sys.exit(1)

    if os.path.exists(legacy_db_path):
        os.remove(legacy_db_path)
        print(f"[INFO] 已存在同名文件，已删除: {legacy_db_path}")

    new_conn = sqlite3.connect(new_db_path)
    old_conn = sqlite3.connect(legacy_db_path)

    new_cur = new_conn.cursor()
    old_cur = old_conn.cursor()

    # ── 1. 创建旧版 schema ──────────────────────────────────────
    old_schema = """
    CREATE TABLE IF NOT EXISTS users (
        id INTEGER NOT NULL,
        screen_name VARCHAR NOT NULL,
        name VARCHAR NOT NULL,
        protected BOOLEAN NOT NULL,
        friends_count INTEGER NOT NULL,
        PRIMARY KEY (id),
        UNIQUE (screen_name)
    );

    CREATE TABLE IF NOT EXISTS user_previous_names (
        id INTEGER NOT NULL,
        uid INTEGER NOT NULL,
        screen_name VARCHAR NOT NULL,
        name VARCHAR NOT NULL,
        record_date DATE NOT NULL,
        PRIMARY KEY (id),
        FOREIGN KEY(uid) REFERENCES users(id)
    );

    CREATE TABLE IF NOT EXISTS lsts (
        id INTEGER NOT NULL,
        name VARCHAR NOT NULL,
        owner_uid INTEGER NOT NULL,
        PRIMARY KEY (id)
    );

    CREATE TABLE IF NOT EXISTS lst_entities (
        id INTEGER NOT NULL,
        lst_id INTEGER NOT NULL,
        name VARCHAR NOT NULL,
        parent_dir VARCHAR NOT NULL COLLATE NOCASE,
        PRIMARY KEY (id),
        UNIQUE (lst_id, parent_dir)
    );

    CREATE TABLE IF NOT EXISTS user_entities (
        id INTEGER NOT NULL,
        user_id INTEGER NOT NULL,
        name VARCHAR NOT NULL,
        latest_release_time DATETIME,
        parent_dir VARCHAR COLLATE NOCASE NOT NULL,
        media_count INTEGER,
        PRIMARY KEY (id),
        UNIQUE (user_id, parent_dir),
        FOREIGN KEY(user_id) REFERENCES users(id)
    );

    CREATE TABLE IF NOT EXISTS user_links (
        id INTEGER NOT NULL,
        user_id INTEGER NOT NULL,
        name VARCHAR NOT NULL,
        parent_lst_entity_id INTEGER NOT NULL,
        PRIMARY KEY (id),
        UNIQUE (user_id, parent_lst_entity_id),
        FOREIGN KEY(user_id) REFERENCES users(id),
        FOREIGN KEY(parent_lst_entity_id) REFERENCES lst_entities(id)
    );

    CREATE INDEX IF NOT EXISTS idx_user_links_user_id ON user_links(user_id);
    """
    old_cur.executescript(old_schema)

    # ── 2. 逐表迁移数据 ────────────────────────────────────────

    # 2a. users — 丢弃 is_accessible 列
    print("[1/6] 迁移 users ...")
    rows = new_cur.execute("SELECT id, screen_name, name, protected, friends_count FROM users").fetchall()
    old_cur.executemany(
        "INSERT INTO users(id, screen_name, name, protected, friends_count) VALUES(?,?,?,?,?)",
        rows,
    )
    print(f"       → {len(rows)} 条记录")

    # 2b. user_previous_names — user_id → uid
    print("[2/6] 迁移 user_previous_names (user_id → uid) ...")
    rows = new_cur.execute("SELECT id, user_id, screen_name, name, record_date FROM user_previous_names").fetchall()
    old_cur.executemany(
        "INSERT INTO user_previous_names(id, uid, screen_name, name, record_date) VALUES(?,?,?,?,?)",
        rows,
    )
    print(f"       → {len(rows)} 条记录")

    # 2c. lsts — owner_user_id → owner_uid
    print("[3/6] 迁移 lsts (owner_user_id → owner_uid) ...")
    rows = new_cur.execute("SELECT id, name, owner_user_id FROM lsts").fetchall()
    old_cur.executemany(
        "INSERT INTO lsts(id, name, owner_uid) VALUES(?,?,?)",
        rows,
    )
    print(f"       → {len(rows)} 条记录")

    # 2d. lst_entities — 直接复制
    print("[4/6] 迁移 lst_entities ...")
    rows = new_cur.execute("SELECT id, lst_id, name, parent_dir FROM lst_entities").fetchall()
    old_cur.executemany(
        "INSERT INTO lst_entities(id, lst_id, name, parent_dir) VALUES(?,?,?,?)",
        rows,
    )
    print(f"       → {len(rows)} 条记录")

    # 2e. user_entities — 直接复制
    print("[5/6] 迁移 user_entities ...")
    rows = new_cur.execute(
        "SELECT id, user_id, name, latest_release_time, parent_dir, media_count FROM user_entities"
    ).fetchall()
    old_cur.executemany(
        "INSERT INTO user_entities(id, user_id, name, latest_release_time, parent_dir, media_count) VALUES(?,?,?,?,?,?)",
        rows,
    )
    print(f"       → {len(rows)} 条记录")

    # 2f. user_links — 直接复制
    print("[6/6] 迁移 user_links ...")
    rows = new_cur.execute("SELECT id, user_id, name, parent_lst_entity_id FROM user_links").fetchall()
    old_cur.executemany(
        "INSERT INTO user_links(id, user_id, name, parent_lst_entity_id) VALUES(?,?,?,?)",
        rows,
    )
    print(f"       → {len(rows)} 条记录")

    # ── 3. 收尾 ────────────────────────────────────────────────
    old_conn.commit()

    new_cur.close()
    old_cur.close()
    new_conn.close()
    old_conn.close()

    print(f"\n[DONE] 转换完成，旧版数据库已写入: {legacy_db_path}")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print(__doc__)
        sys.exit(1)
    convert(sys.argv[1], sys.argv[2])
