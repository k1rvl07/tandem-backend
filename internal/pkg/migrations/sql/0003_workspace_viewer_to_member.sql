DO $$
BEGIN
  IF to_regclass('public.workspace_members') IS NOT NULL THEN
    UPDATE workspace_members SET role = 'member' WHERE role = 'viewer';
  END IF;
END $$;