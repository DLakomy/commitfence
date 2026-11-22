CREATE OR REPLACE PROCEDURE app.worker IS
BEGIN
  WHILE ready LOOP COMMIT; END LOOP;
  FOR n IN 1..3 LOOP ROLLBACK; END LOOP;
  LOOP ROLLBACK TO SAVEPOINT s; EXIT; END LOOP;
  commit$work;
  rollback#work;
  commit#work;
  rollback$work;
  $commit;
  #rollback;
  <<section_a>> BEGIN NULL; END section_a;
  COMMIT;
  <<worker>> DECLARE x NUMBER;
  BEGIN
    <<inner_block>> BEGIN ROLLBACK; END inner_block;
  END worker;
  COMMIT; <<worker>> BEGIN BEGIN NULL; END; END;
END worker;
/
CREATE OR REPLACE PROCEDURE app.bad IS
BEGIN
  <<section_a>> BEGIN NULL; END wrong_label;
END bad;
/
