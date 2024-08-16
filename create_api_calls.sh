#!/bin/bash

url="http://localhost:8080/user"

for i in {1..100}
do
  # Prepare the JSON data
  json=$(cat <<EOF
{
  "id": "user_$i",
  "name": "User $i",
  "phone_no": $((1000000000 + i))
}
EOF
)

  # Make the POST request
  curl -X POST -H "Content-Type: application/json" -d "$json" "$url"
  echo "Request $i sent."
done
