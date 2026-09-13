DO $$
BEGIN
  IF to_regclass('public.tasks') IS NOT NULL THEN
    UPDATE tasks SET author_id = NULL WHERE author_id IS NOT NULL AND author_id::text !~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$';
    UPDATE tasks SET assignee_id = NULL WHERE assignee_id IS NOT NULL AND assignee_id::text !~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$';
    UPDATE tasks SET curator_id = NULL WHERE curator_id IS NOT NULL AND curator_id::text !~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$';
    UPDATE tasks SET parent_id = NULL WHERE parent_id IS NOT NULL AND parent_id::text !~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$';
    ALTER TABLE tasks ALTER COLUMN author_id TYPE uuid USING author_id::uuid;
    ALTER TABLE tasks ALTER COLUMN assignee_id TYPE uuid USING assignee_id::uuid;
    ALTER TABLE tasks ALTER COLUMN curator_id TYPE uuid USING curator_id::uuid;
    ALTER TABLE tasks ALTER COLUMN parent_id TYPE uuid USING parent_id::uuid;
    DELETE FROM tasks t WHERE NOT EXISTS (SELECT 1 FROM board_columns c WHERE c.id = t.column_id);
  END IF;
  IF to_regclass('public.task_attachments') IS NOT NULL THEN
    UPDATE task_attachments SET uploaded_by = NULL WHERE uploaded_by IS NOT NULL AND uploaded_by::text !~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$';
    ALTER TABLE task_attachments ALTER COLUMN uploaded_by DROP NOT NULL;
    ALTER TABLE task_attachments ALTER COLUMN uploaded_by TYPE uuid USING uploaded_by::uuid;
    DELETE FROM task_attachments a WHERE NOT EXISTS (SELECT 1 FROM tasks t WHERE t.id = a.task_id);
  END IF;
  IF to_regclass('public.board_columns') IS NOT NULL THEN
    DELETE FROM board_columns c WHERE NOT EXISTS (SELECT 1 FROM boards b WHERE b.id = c.board_id);
  END IF;
  IF to_regclass('public.boards') IS NOT NULL THEN
    DELETE FROM boards b WHERE NOT EXISTS (SELECT 1 FROM workspaces w WHERE w.id = b.workspace_id);
  END IF;
  IF to_regclass('public.workspace_members') IS NOT NULL THEN
    DELETE FROM workspace_members m WHERE NOT EXISTS (SELECT 1 FROM workspaces w WHERE w.id = m.workspace_id) OR NOT EXISTS (SELECT 1 FROM users u WHERE u.id = m.user_id);
  END IF;
END $$;