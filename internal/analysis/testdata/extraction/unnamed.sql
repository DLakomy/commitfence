CREATE OR REPLACE NONEDITIONABLE PACKAGE app.unnamed_api AS
  --@commitfence
  PROCEDURE permitted(n NUMBER);
  PROCEDURE driver;
END;
/
CREATE OR REPLACE EDITIONABLE PACKAGE BODY app.unnamed_api AS
  PROCEDURE permitted(n NUMBER) IS BEGIN COMMIT; END;
  PROCEDURE permitted IS BEGIN ROLLBACK; END permitted;
  FUNCTION leaf RETURN NUMBER IS BEGIN COMMIT; RETURN 1; END;
  PROCEDURE middle IS n NUMBER; BEGIN n := leaf(); END;
  PROCEDURE driver IS BEGIN middle; END driver;
  PROCEDURE quiet IS BEGIN NULL; END;
BEGIN NULL; END;
/
CREATE FUNCTION app.unnamed_top RETURN NUMBER IS
BEGIN
  app.unnamed_api.driver;
  RETURN 0;
END;
/
CREATE PROCEDURE app.unnamed_host IS
  PROCEDURE logger IS
    PRAGMA AUTONOMOUS_TRANSACTION;
    PROCEDURE helper IS BEGIN COMMIT; END;
  BEGIN
    helper;
    EXECUTE IMMEDIATE 'unavailable SQL';
  END;
  PROCEDURE quiet IS BEGIN NULL; END;
BEGIN
  logger;
END unnamed_host;
/
CREATE PROCEDURE app.unnamed_host_caller IS BEGIN app.unnamed_host; END;
/
CREATE PROCEDURE app.unnamed_blocks IS
  n NUMBER := CASE WHEN 1 = 1 THEN CASE 1 WHEN 1 THEN 2 END ELSE 0 END;
  procedure_name NUMBER;
  function_count NUMBER;
BEGIN
  BEGIN
    NULL;
  EXCEPTION WHEN OTHERS THEN
    ROLLBACK;
  END;
  COMMIT;
  <<unnamed_blocks>> DECLARE
    PROCEDURE local_helper IS
      PROCEDURE leaf IS BEGIN ROLLBACK; END;
    BEGIN NULL; END;
  BEGIN
    <<"END;">> BEGIN NULL; END "END;";
    IF n > 0 THEN
      <<items>> FOR i IN 1..2 LOOP
        CASE i
          WHEN 1 THEN SELECT CASE WHEN n = 2 THEN CASE n WHEN 2 THEN 3 END ELSE 4 END + 1 INTO n FROM dual;
          ELSE NULL;
        END CASE;
      END LOOP;
    END IF;
    local_helper;
  EXCEPTION WHEN OTHERS THEN
    NULL;
  END unnamed_blocks;
  ROLLBACK;
END;
/
