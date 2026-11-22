CREATE PROCEDURE app.audit IS
  PRAGMA AUTONOMOUS_TRANSACTION;
BEGIN
  COMMIT;
  ext.audit_unknown;
END audit;
/
CREATE PROCEDURE app.context IS
  PRAGMA AUTONOMOUS_TRANSACTION;
  PROCEDURE ordinary IS
  BEGIN
    ROLLBACK;
    ext.ordinary_unknown;
  END ordinary;
  PROCEDURE isolated IS
    PRAGMA AUTONOMOUS_TRANSACTION;
  BEGIN
    COMMIT;
    ext.isolated_unknown;
  END isolated;
BEGIN
  ordinary;
  isolated;
END context;
/
CREATE PROCEDURE app.host IS
  PROCEDURE helper IS
    PRAGMA AUTONOMOUS_TRANSACTION;
  BEGIN
    app.audit;
  END helper;
BEGIN
  COMMIT;
  ext.host_unknown;
  helper;
END host;
/
CREATE PACKAGE app.capability AS
  --@commitfence
  PROCEDURE isolated;
END capability;
/
CREATE PACKAGE BODY app.capability AS
  PROCEDURE isolated IS
    PRAGMA AUTONOMOUS_TRANSACTION;
  BEGIN
    ext.capability_unknown;
  END isolated;
END capability;
/
CREATE PROCEDURE app.mixed IS
  PRAGMA AUTONOMOUS_TRANSACTION;
BEGIN
  COMMIT;
  ext.autonomous_unknown;
END mixed;
/
CREATE PROCEDURE app.mixed(n NUMBER) IS
BEGIN
  ROLLBACK;
  ext.mixed_unknown;
END mixed;
/
CREATE PROCEDURE app.caller IS
BEGIN
  app.audit;
  app.context;
  app.host;
  app.capability.isolated;
  app.mixed;
END caller;
/
