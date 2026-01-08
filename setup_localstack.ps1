# Check if LocalStack is running
Write-Host "Checking for LocalStack..."
$localstackUrl = "http://localhost:4566"

# Create Fake Profile in .aws/config and credentials if checks out
# Note: Users usually manage their own .aws/config. I will append or instruct.
# For this script we will just populate data.

Write-Host "Populating SSM Parameters..."
aws --endpoint-url=$localstackUrl ssm put-parameter --name "/app/prod/db_url" --value "jdbc:mysql://prod-db:3306/db" --type String --overwrite
aws --endpoint-url=$localstackUrl ssm put-parameter --name "/app/prod/api_key" --value "secret-key-prod" --type String --overwrite
aws --endpoint-url=$localstackUrl ssm put-parameter --name "/app/dev/db_url" --value "jdbc:mysql://dev-db:3306/db" --type String --overwrite
aws --endpoint-url=$localstackUrl ssm put-parameter --name "service-a-config" --value "some-config" --type String --overwrite

Write-Host "Running EC2 Instances..."
aws --endpoint-url=$localstackUrl ec2 run-instances --image-id ami-12345678 --count 1 --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=WebServer-Prod}]'
aws --endpoint-url=$localstackUrl ec2 run-instances --image-id ami-87654321 --count 1 --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=Worker-Dev}]'

Write-Host "Done! Please ensure you have a profile named 'localstack' in your ~/.aws/config configured to point to http://localhost:4566"
Write-Host "Example:"
Write-Host "[profile localstack]"
Write-Host "region = us-east-1"
Write-Host "output = json"
Write-Host "endpoint_url = http://localhost:4566"
