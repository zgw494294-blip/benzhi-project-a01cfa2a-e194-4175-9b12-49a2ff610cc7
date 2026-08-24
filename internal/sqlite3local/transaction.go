package sqlite3local

type Tx struct {
	conn *Conn
	done bool
}

func (t *Tx) Commit() error {
	if t.done {
		return nil
	}
	err := t.conn.execRaw(`COMMIT`)
	if err == nil {
		t.done = true
	}
	return err
}

func (t *Tx) Rollback() error {
	if t.done {
		return nil
	}
	err := t.conn.execRaw(`ROLLBACK`)
	if err == nil {
		t.done = true
	}
	return err
}
