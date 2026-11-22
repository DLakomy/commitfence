CREATE OR REPLACE PROCEDURE app.outer_r IS
  PROCEDURE logger IS
    PRAGMA AUTONOMOUS_TRANSACTION;
    PROCEDURE ordinary_helper IS
    BEGIN
      COMMIT;
    END ordinary_helper;
  BEGIN
    SAVEPOINT before_work;
    ROLLBACK TO before_work;
    ROLLBACK;
  END logger;
BEGIN
  NULL;
END outer_r;
/
