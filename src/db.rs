use rusqlite::Connection;

pub struct DB {
    conn: Connection
}

impl DB {
    pub fn new(filename: &str) -> DB {
        let conn = Connection::open(filename).expect("Unable to open database");
        conn.execute("CREATE TABLE IF NOT EXISTS pad (id TEXT, data TEXT);", []).unwrap();
        DB {
            conn
        }
    }
    pub fn get(&self, id: &str) -> Option<String> {
        self.conn.query_row("SELECT data FROM pad WHERE id = ?1", &[&id], |row| {
            row.get(0)
        }).ok()
    }

    pub fn set(&self, id: &str, data: &str) {
        self.conn.execute("INSERT OR REPLACE INTO pad (id, data) VALUES (?1, ?2)", &[&id, &data]).unwrap();
    }

    pub fn delete(&self, id: &str) {
        self.conn.execute("DELETE FROM pad WHERE id = ?1", &[&id]).unwrap();
    }
}