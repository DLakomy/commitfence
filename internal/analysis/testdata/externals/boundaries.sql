CREATE PACKAGE app.api IS
 --@commitfence
 PROCEDURE advertised;
END api;
/
CREATE PROCEDURE app.auto IS
 PRAGMA AUTONOMOUS_TRANSACTION;
BEGIN
 COMMIT;
 ext.unknown;
END auto;
/
CREATE PROCEDURE app.empty_body IS
BEGIN
 NULL;
END empty_body;
/
CREATE PROCEDURE app.wrapper IS
BEGIN
 app.api.advertised;
 app.auto;
 app.empty_body;
END wrapper;
/
CREATE PROCEDURE app.unknown_body IS
BEGIN
 ext.unknown;
END unknown_body;
/
