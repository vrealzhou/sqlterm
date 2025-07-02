# Debug Guide for Integer Type Issues

## Problem
Still seeing "Unknown type" instead of integer values when running `select * from posts`.

## Enhanced Fix Applied
The new type conversion logic now:

1. **Type Name Matching**: Explicitly matches PostgreSQL type names like "INT4", "INTEGER", "BIGINT"
2. **Detailed Error Reporting**: Shows exactly what type and conversion failed
3. **Comprehensive Fallback**: Tries multiple integer sizes if type name matching fails

## To Debug the Issue

### Step 1: Check Actual Type Names
Run this query to see what PostgreSQL reports as the column types:
```sql
SELECT column_name, data_type, udt_name 
FROM information_schema.columns 
WHERE table_name = 'posts' 
ORDER BY ordinal_position;
```

### Step 2: Run the Fixed Version
The new implementation will now show more specific error messages like:
- `"Failed to parse INT4 as i32"` - Shows the exact type name and expected conversion
- `"Unknown PostgreSQL type: SERIAL"` - Shows unhandled type names

### Step 3: Look for Error Patterns
With the improved error reporting, you should see messages like:
- If you see `"Failed to parse INT4 as i32"` - The type detection works but conversion fails
- If you see `"Unknown PostgreSQL type: XYZ"` - We need to add that type to our mapping

## Common PostgreSQL Integer Types
The fix now handles:
- `INT2`, `SMALLINT` → `i16`
- `INT4`, `INTEGER`, `INT` → `i32` 
- `INT8`, `BIGINT` → `i64`

## If Still Not Working
The error messages will now tell us exactly:
1. What PostgreSQL type name is being reported
2. Which Rust type conversion is failing
3. Whether it's a type mapping issue or a conversion issue

## Expected Behavior After Fix
Instead of generic "Unknown type", you should now see either:
- **Success**: Proper integer values displayed
- **Specific Error**: Detailed message about what failed (e.g., "Failed to parse SERIAL as i32")

This will help us identify the exact issue and create a targeted fix.

## Test Command
```bash
cargo run --package ui
# Then connect to your PostgreSQL database
# Run: select * from posts
# Look at the specific error messages in the Unknown type fields
```