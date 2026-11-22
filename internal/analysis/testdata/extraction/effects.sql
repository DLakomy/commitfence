CREATE OR REPLACE PROCEDURE app.direct IS
BEGIN
  COMMIT WORK WRITE IMMEDIATE WAIT;
  IF 1 = 1 THEN ROLLBACK; ELSE ROLLBACK TO point_a; END IF;
  BEGIN
    SAVEPOINT point_a;
    INSERT INTO things VALUES (1);
  EXCEPTION WHEN OTHERS THEN
    ROLLBACK TO SAVEPOINT point_a;
  END;
END direct;
/
CREATE OR REPLACE FUNCTION app.clean RETURN NUMBER AS
  text_value VARCHAR2(100) := 'COMMIT; '' ROLLBACK; --@commitfence';
BEGIN
  -- COMMIT;
  /* ROLLBACK; --@commitfence */
  text_value := q'[COMMIT; ' END clean; --@commitfence]';
  text_value := Q'{ROLLBACK; PROCEDURE bogus IS BEGIN}';
  text_value := q'<COMMIT;>';
  text_value := q'!ROLLBACK;!';
  SAVEPOINT point_a;
  UPDATE things SET value = 2;
  DELETE FROM things;
  RETURN 1;
END clean;
/
-- Example: --@commitfence
CREATE OR REPLACE PROCEDURE app.prose IS
BEGIN
  COMMIT;
END prose;
/
CREATE OR REPLACE PROCEDURE app.literal_marker IS
  x VARCHAR2(30) := '--@commitfence';
BEGIN
  ROLLBACK;
END literal_marker;
/
/*
--@commitfence
*/
CREATE OR REPLACE PROCEDURE app.block_marker IS
BEGIN
  COMMIT;
END block_marker;
/
