CREATE OR REPLACE PACKAGE BODY app.import_api AS
  PROCEDURE run_import IS
  BEGIN
    app.customer_service.save_customer(42);
    ext.customer_api.flush_batch;
  END run_import;
END import_api;
/
