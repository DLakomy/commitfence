CREATE OR REPLACE PACKAGE BODY app.split AS
  PROCEDURE permitted IS
  BEGIN
    COMMIT;
  END;
  PROCEDURE other_member IS
  BEGIN
    ROLLBACK;
  END other_member;
END;
/
