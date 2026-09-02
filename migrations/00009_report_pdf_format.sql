-- +goose Up

ALTER TABLE reports DROP CONSTRAINT reports_format_check;
ALTER TABLE reports ADD CONSTRAINT reports_format_check
    CHECK (format = ANY (ARRAY['csv'::text, 'json'::text, 'pdf'::text]));

ALTER TABLE report_payloads DROP CONSTRAINT report_payloads_content_type_check;
ALTER TABLE report_payloads ADD CONSTRAINT report_payloads_content_type_check
    CHECK (content_type = ANY (ARRAY['application/json'::text, 'text/csv'::text, 'application/pdf'::text]));

-- +goose Down

ALTER TABLE report_payloads DROP CONSTRAINT report_payloads_content_type_check;
ALTER TABLE report_payloads ADD CONSTRAINT report_payloads_content_type_check
    CHECK (content_type = ANY (ARRAY['application/json'::text, 'text/csv'::text]));

ALTER TABLE reports DROP CONSTRAINT reports_format_check;
ALTER TABLE reports ADD CONSTRAINT reports_format_check
    CHECK (format = ANY (ARRAY['csv'::text, 'json'::text]));
