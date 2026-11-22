--@commitfence
CREATE PROCEDURE app.native IS BEGIN
  EXECUTE IMMEDIATE 'COMMIT';
  EXECUTE IMMEDIATE q'[TRUNCATE TABLE t]';
  EXECUTE IMMEDIATE sql_text;
  OPEN c FOR 'SELECT 1 FROM dual';
  OPEN c FOR sql_text;
  OPEN c FOR SELECT 1 FROM dual;
END native;
/
CREATE PROCEDURE app.dbms_dynamic IS BEGIN
  DBMS_SQL.PARSE(c, 'COMMIT', 1);
  n := DBMS_SQL.EXECUTE(c);
  n := DBMS_SQL.EXECUTE_AND_FETCH(c);
END dbms_dynamic;
/
CREATE PACKAGE app.api IS
  PROCEDURE undescribed;
END api;
/
CREATE PROCEDURE app.mixed IS BEGIN
  COMMIT;
  app.api.undescribed;
  Ext . Missing();
END mixed;
/
--@commitfence
CREATE PROCEDURE app.wrapper IS BEGIN app.mixed; END wrapper;
/
CREATE PROCEDURE app.top IS BEGIN app.wrapper; END top;
/
CREATE PROCEDURE app.cycle_a IS BEGIN app.cycle_b; END cycle_a;
/
CREATE PROCEDURE app.cycle_b IS BEGIN app.cycle_a; app.native; END cycle_b;
/
CREATE PROCEDURE app.builtins IS BEGIN
  n := ABS(-1) + NVL(NULL, 0);
  DBMS_OUTPUT.PUT_LINE('ok');
  DBMS_RANDOM.VALUE();
  DBMS_OUTPUT.UNDESCRIBED();
END builtins;
/
CREATE FUNCTION ABS(n NUMBER) RETURN NUMBER IS BEGIN missing_dependency(); RETURN n; END ABS;
/
CREATE PROCEDURE app.isolated IS
  PRAGMA AUTONOMOUS_TRANSACTION;
BEGIN EXECUTE IMMEDIATE 'COMMIT'; END isolated;
/
CREATE PROCEDURE app.isolated_caller IS BEGIN app.isolated; END isolated_caller;
/
CREATE PROCEDURE app.nested IS
  PROCEDURE helper IS BEGIN OPEN c FOR sql_text; END helper;
BEGIN NULL; END nested;
/
