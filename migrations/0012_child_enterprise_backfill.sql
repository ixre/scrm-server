-- Older implicit association saves set parent IDs but could omit ext_corp_id.
-- Backfill only empty enterprise fields from an unambiguous primary-key parent.
-- Existing nonempty enterprise IDs and all business IDs remain unchanged.
UPDATE quick_reply c SET ext_corp_id = p.ext_corp_id FROM quick_reply_group p WHERE c.group_id = p.id AND COALESCE(trim(c.ext_corp_id), '') = '';
UPDATE quick_reply_detail c SET ext_corp_id = p.ext_corp_id FROM quick_reply p WHERE c.quick_reply_id = p.id AND COALESCE(trim(c.ext_corp_id), '') = '';
UPDATE welcome_msg c SET ext_corp_id = p.ext_corp_id FROM welcome_msg p WHERE c.main_welcome_msg_id = p.id AND COALESCE(trim(c.ext_corp_id), '') = '';
UPDATE contact_way_staff c SET ext_corp_id = p.ext_corp_id FROM contact_way p WHERE c.contact_way_id = p.id AND COALESCE(trim(c.ext_corp_id), '') = '';
UPDATE contact_way_backup_staff c SET ext_corp_id = p.ext_corp_id FROM contact_way p WHERE c.contact_way_id = p.id AND COALESCE(trim(c.ext_corp_id), '') = '';
UPDATE contact_way_schedule c SET ext_corp_id = p.ext_corp_id FROM contact_way p WHERE c.contact_way_id = p.id AND COALESCE(trim(c.ext_corp_id), '') = '';
UPDATE contact_way_schedule_staff c SET ext_corp_id = p.ext_corp_id FROM contact_way_schedule p WHERE c.contact_way_schedule_id = p.id AND c.contact_way_id = p.contact_way_id AND COALESCE(trim(c.ext_corp_id), '') = '';
UPDATE mass_msg_staff c SET ext_corp_id = p.ext_corp_id FROM mass_msg p WHERE c.mass_msg_id = p.id AND COALESCE(trim(c.ext_corp_id), '') = '';
