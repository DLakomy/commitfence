CREATE PACKAGE bare AS
  --@commitfence
  PROCEDURE advertised;
END bare;
/
CREATE PROCEDURE caller IS BEGIN advertised; bare.advertised; END caller;
/
CREATE PROCEDURE app.caller IS BEGIN advertised; bare.advertised; other.advertised; END caller;
/
CREATE PROCEDURE app.outer_r IS
  PROCEDURE helper IS BEGIN COMMIT; END helper;
BEGIN
  outer_r.helper;
END outer_r;
/
CREATE PROCEDURE app.quoted IS
  n NUMBER;
BEGIN
  n := "unknown f()";
  n := ext . unknown ();
END quoted;
/
