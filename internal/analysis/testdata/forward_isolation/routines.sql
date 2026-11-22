CREATE PROCEDURE app.isolated_parent IS
  --@commitfence
  PROCEDURE helper;
  PROCEDURE helper IS
    PRAGMA AUTONOMOUS_TRANSACTION;
  BEGIN
    COMMIT;
  END;
BEGIN
  helper;
END;
/
CREATE PROCEDURE app.isolated_caller IS BEGIN app.isolated_parent; END isolated_caller;
/
CREATE PROCEDURE app.mixed_parent IS
  --@commitfence
  PROCEDURE helper;
  PROCEDURE helper IS
    PRAGMA AUTONOMOUS_TRANSACTION;
  BEGIN
    NULL;
  END;
  PROCEDURE helper(n NUMBER) IS BEGIN NULL; END;
BEGIN
  NULL;
END mixed_parent;
/
CREATE PROCEDURE app.mixed_caller IS BEGIN app.mixed_parent; END;
/
