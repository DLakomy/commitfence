--@commitfence
CREATE PROCEDURE app.host.helper IS BEGIN NULL; END helper;
/
CREATE PROCEDURE app.host IS
  PROCEDURE worker;
  PROCEDURE worker IS BEGIN helper; END worker;
  PROCEDURE helper IS BEGIN COMMIT; END helper;
BEGIN
  worker;
END host;
/
CREATE PROCEDURE app.isolated IS
  PROCEDURE logger IS
    PRAGMA AUTONOMOUS_TRANSACTION;
  BEGIN
    app.host;
    unknown_ext();
  END logger;
BEGIN
  logger;
END isolated;
/
CREATE PROCEDURE app.isolated_caller IS BEGIN app.isolated; END isolated_caller;
/
CREATE PROCEDURE app.mixed IS PRAGMA AUTONOMOUS_TRANSACTION; BEGIN COMMIT; END mixed;
/
CREATE PROCEDURE app.mixed(n NUMBER) IS BEGIN COMMIT; END mixed;
/
CREATE PROCEDURE app.mixed_caller IS BEGIN app.mixed; END mixed_caller;
/
CREATE FUNCTION "app"."Count" RETURN NUMBER IS BEGIN COMMIT; RETURN 1; END "Count";
/
CREATE FUNCTION app.count RETURN NUMBER IS BEGIN COMMIT; RETURN 1; END count;
/
CREATE PROCEDURE app.quoted_calls IS
  n NUMBER;
BEGIN
  n := "app"."Count"();
  n := "app"."count"();
  n := count();
  n := ABS(1);
END quoted_calls;
/
