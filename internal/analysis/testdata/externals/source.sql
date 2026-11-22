CREATE PROCEDURE app.caller IS
BEGIN
 ext.api.safe_call;
 ext.api.unsafe_call;
 ext.api.missing_call;
 ext."Mixed".run;
 ext.mixed.run;
 app.conflict;
 app.uncertain;
 app.isolated;
 app.declared;
END caller;
/
CREATE PROCEDURE app.conflict IS
BEGIN
 COMMIT;
END conflict;
/
CREATE PROCEDURE app.uncertain IS
BEGIN
 ext.other.run;
END uncertain;
/
CREATE PROCEDURE app.isolated IS
 PRAGMA AUTONOMOUS_TRANSACTION;
BEGIN
 COMMIT;
 ext.other.run;
END isolated;
/
CREATE PACKAGE app IS
 PROCEDURE declared;
END app;
/
