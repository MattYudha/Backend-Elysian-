# Database Schema Documentation

This document provides documentation for the Elysian Backend PostgreSQL database schema.

## Overview

The Elysian Backend uses PostgreSQL as its primary database with the following key components:
- **User Management**: Authentication and authorization system
- **Workflow Management**: Versioned workflows with node-based execution
- **Audit System**: Comprehensive logging of system activities
- **File Storage**: Document and asset management

## Schema Design Principles

- **UUID Primary Keys**: All entities use UUID v4 for primary keys
- **Soft Deletes**: Most tables include `deleted_at` timestamp for soft deletion
- **Timestamps**: Automatic `created_at` and `updated_at` with triggers
- **JSONB Fields**: Flexible configuration storage using PostgreSQL's JSONB type
- **Proper Indexing**: Strategic indexes for performance optimization
- **Referential Integrity**: Foreign key constraints with appropriate cascade rules

## Core Tables

### Users (`users`)

The central user management table storing authentication and profile information.

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    avatar_url VARCHAR(500),
    is_active BOOLEAN DEFAULT TRUE NOT NULL,
    email_verified_at TIMESTAMP,
    last_login_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP,

    CONSTRAINT uq_users_email UNIQUE(email),
    CONSTRAINT chk_users_email CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$')
);
```

**Key Features:**
- Email validation with regex constraint
- Soft deletion support
- Automatic `updated_at` trigger
- Unique email constraint

**Indexes:**
- `idx_users_email` on `(email)` where `deleted_at IS NULL`
- `idx_users_created_at` on `(created_at)`
- `idx_users_deleted_at` on `(deleted_at)` where `deleted_at IS NOT NULL`

### Roles and Permissions (`roles`, `user_roles`)

Role-based access control system.

```sql
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL,
    description TEXT,
    permissions JSONB DEFAULT '[]'::jsonb NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,

    CONSTRAINT uq_roles_name UNIQUE(name)
);

CREATE TABLE user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    role_id UUID NOT NULL,
    assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    assigned_by UUID,

    CONSTRAINT fk_user_roles_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_roles_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    CONSTRAINT uq_user_roles_unique UNIQUE(user_id, role_id)
);
```

**Key Features:**
- Flexible permission system using JSONB
- User-role assignment tracking
- Unique user-role combinations

### Workflows (`workflows`, `workflow_versions`)

Version-controlled workflow management system.

```sql
CREATE TABLE workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    version INTEGER DEFAULT 1 NOT NULL,
    status VARCHAR(50) DEFAULT 'draft' NOT NULL,
    is_template BOOLEAN DEFAULT FALSE NOT NULL,
    is_public BOOLEAN DEFAULT FALSE NOT NULL,
    tags TEXT[],
    configuration JSONB DEFAULT '{}'::jsonb NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP,

    CONSTRAINT fk_workflows_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT chk_workflows_status CHECK (status IN ('draft', 'published', 'archived'))
);

CREATE TABLE workflow_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL,
    version INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    configuration JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_by UUID NOT NULL,

    CONSTRAINT fk_workflow_versions_workflow FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE,
    CONSTRAINT fk_workflow_versions_user FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT uq_workflow_versions_unique UNIQUE(workflow_id, version)
);
```

**Key Features:**
- Version control for workflow changes
- Template and public workflow support
- Tag-based organization
- Status management (draft/published/archived)

### Workflow Structure (`workflow_nodes`, `workflow_edges`)

Graph-based workflow definition using nodes and edges.

```sql
CREATE TABLE workflow_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL,
    node_key VARCHAR(100) NOT NULL,
    node_type VARCHAR(100) NOT NULL,
    label VARCHAR(255),
    position_x REAL,
    position_y REAL,
    configuration JSONB DEFAULT '{}'::jsonb NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,

    CONSTRAINT fk_workflow_nodes_workflow FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE,
    CONSTRAINT uq_workflow_nodes_key UNIQUE(workflow_id, node_key)
);

CREATE TABLE workflow_edges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL,
    source_node_key VARCHAR(100) NOT NULL,
    target_node_key VARCHAR(100) NOT NULL,
    edge_type VARCHAR(50) DEFAULT 'default' NOT NULL,
    configuration JSONB DEFAULT '{}'::jsonb NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,

    CONSTRAINT fk_workflow_edges_workflow FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE,
    CONSTRAINT chk_workflow_edges_different_nodes CHECK (source_node_key != target_node_key)
);
```

**Key Features:**
- Node-based workflow definition
- Visual positioning support (x,y coordinates)
- Edge configuration for complex workflows
- Unique node keys within workflows

### Execution Engine (`executions`, `execution_logs`)

Workflow execution tracking and logging.

```sql
CREATE TABLE executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL,
    user_id UUID NOT NULL,
    workflow_version INTEGER,
    status VARCHAR(50) DEFAULT 'pending' NOT NULL,
    input_data JSONB,
    output_data JSONB,
    error_message TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,

    CONSTRAINT fk_executions_workflow FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE,
    CONSTRAINT fk_executions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT chk_executions_status CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled'))
);

CREATE TABLE execution_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL,
    node_key VARCHAR(100),
    log_level VARCHAR(20) DEFAULT 'info' NOT NULL,
    message TEXT NOT NULL,
    data JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,

    CONSTRAINT fk_execution_logs_execution FOREIGN KEY (execution_id) REFERENCES executions(id) ON DELETE CASCADE,
    CONSTRAINT chk_execution_logs_level CHECK (log_level IN ('debug', 'info', 'warn', 'error'))
);
```

**Key Features:**
- Execution status tracking
- Input/output data storage
- Detailed execution logs per node
- Version-specific execution

### File Management (`files`)

Document and asset storage system.

```sql
CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    filename VARCHAR(255) NOT NULL,
    original_filename VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    file_size BIGINT NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    checksum VARCHAR(128),
    is_public BOOLEAN DEFAULT FALSE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP,

    CONSTRAINT fk_files_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT chk_files_size CHECK (file_size >= 0)
);
```

**Key Features:**
- File metadata storage
- Checksum for integrity verification
- Public/private file access control
- Soft deletion support

### Authentication (`refresh_tokens`)

JWT refresh token management.

```sql
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP,

    CONSTRAINT fk_refresh_tokens_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT uq_refresh_tokens_hash UNIQUE(token_hash)
);
```

**Key Features:**
- Secure token hash storage
- Expiration and revocation tracking
- User association for cleanup

### Audit System (`audit_logs`)

Comprehensive activity logging for compliance and debugging.

```sql
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(100) NOT NULL,
    entity_id UUID,
    changes JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,

    CONSTRAINT fk_audit_logs_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);
```

**Key Features:**
- Entity-level change tracking
- IP address and user agent logging
- JSONB change history
- Nullable user_id for system actions

## Database Design Patterns

### Soft Deletes
Most tables implement soft deletion using `deleted_at` timestamp:
- Allows data recovery
- Maintains referential integrity
- Requires `WHERE deleted_at IS NULL` in queries

### Automatic Timestamps
All tables use triggers for automatic timestamp management:
- `created_at`: Set on insert only
- `updated_at`: Updated on every row modification

### UUID Primary Keys
- Globally unique identifiers
- No collision risks in distributed systems
- Better for security (no sequential ID enumeration)

### JSONB Configuration
Flexible configuration storage for:
- Workflow node settings
- User permissions
- Audit change tracking
- Input/output data

## Migration Strategy

The project uses [goose](https://github.com/pressly/goose) for database migrations:

```bash
# Create new migration
make migrate-create NAME=migration_name

# Run migrations
make migrate-up

# Rollback migrations
make migrate-down

# Check status
make migrate-status
```

## Performance Considerations

### Indexing Strategy
- Primary key indexes on all UUID columns
- Foreign key indexes for join performance
- Partial indexes for soft deletes
- GIN indexes for array and JSONB fields
- Composite indexes for common query patterns

### Query Optimization
- Use `EXPLAIN ANALYZE` for query planning
- Implement pagination for large result sets
- Consider read replicas for heavy read loads
- Monitor slow query logs

### Connection Management
- Implement connection pooling
- Set appropriate timeouts
- Monitor connection pool metrics
- Handle connection failures gracefully

## Data Integrity

### Constraints
- Check constraints for valid status values
- Unique constraints for business rules
- Foreign key constraints with appropriate cascade rules
- Email format validation

### Data Validation
- Application-level validation before database insertion
- Database constraints as final safety net
- Input sanitization to prevent SQL injection

## Backup and Recovery

### Backup Strategy
- Regular automated backups
- Point-in-time recovery capability
- Test backup restoration procedures
- Encrypt sensitive backup data

### Disaster Recovery
- Multi-region database replicas
- Automated failover procedures
- Data retention policies
- Compliance with data protection regulations</content>
