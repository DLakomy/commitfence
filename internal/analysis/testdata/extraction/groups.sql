CREATE OR REPLACE PACKAGE app.api AS
  -- description
  --@commitfence
  -- more description
  PROCEDURE permitted;
  FUNCTION untouched RETURN NUMBER;
END api;
/
CREATE OR REPLACE PACKAGE BODY app.api AS
  PROCEDURE permitted IS
    PROCEDURE helper IS
    BEGIN
      COMMIT;
    END helper;
  BEGIN
    ROLLBACK;
  END permitted;
  PROCEDURE permitted(n NUMBER) IS
  BEGIN
    COMMIT;
  END permitted;
  FUNCTION untouched RETURN NUMBER IS
  BEGIN
    ROLLBACK TO SAVEPOINT s;
    RETURN 0;
  END untouched;
END api;
/
CREATE OR REPLACE PROCEDURE app.outer_r IS
  PROCEDURE forward_r;
  --@commitfence
  PROCEDURE forward_r IS
  BEGIN
    COMMIT;
  END forward_r;
BEGIN
  NULL;
END outer_r;
/
CREATE OR REPLACE PROCEDURE other.permitted IS
BEGIN
  COMMIT;
END permitted;
/
--@commitfence
CREATE OR REPLACE PROCEDURE "app"."Case" IS
BEGIN
  COMMIT;
END "Case";
/
CREATE OR REPLACE PROCEDURE "app"."case" IS
BEGIN
  COMMIT;
END "case";
/
