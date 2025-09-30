#!/bin/bash

echo "🚀 Testing PubSubGo System"
echo "=========================="

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

SERVER_URL="http://localhost:8080"
CLI="./pubsub-cli"

# Function to check if server is running
check_server() {
    echo -n "🔍 Checking if server is running... "
    if curl -s -f $SERVER_URL/health > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Server is running${NC}"
        return 0
    else
        echo -e "${RED}❌ Server is not running${NC}"
        echo "Please start the server first:"
        echo "  ./pubsubgo-server -config config.yaml"
        return 1
    fi
}

# Function to test CLI connection
test_cli() {
    echo -n "🔧 Testing CLI connection... "
    if $CLI --server $SERVER_URL topics list > /dev/null 2>&1; then
        echo -e "${GREEN}✅ CLI can connect${NC}"
        return 0
    else
        echo -e "${RED}❌ CLI cannot connect${NC}"
        return 1
    fi
}

# Function to create test topic
create_topic() {
    echo -n "📁 Creating test topic... "
    if $CLI --server $SERVER_URL topics create --name test-topic --partitions 2 > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Topic created${NC}"
        return 0
    else
        echo -e "${YELLOW}⚠️  Topic might already exist${NC}"
        return 0
    fi
}

# Function to publish test message
publish_message() {
    echo -n "📤 Publishing test message... "
    if $CLI --server $SERVER_URL publish --topic test-topic --message "Hello from test!" --key "test-key" --priority "high" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Message published${NC}"
        return 0
    else
        echo -e "${RED}❌ Failed to publish message${NC}"
        return 1
    fi
}

# Function to list topics
list_topics() {
    echo "📋 Listing topics:"
    $CLI --server $SERVER_URL topics list
}

# Function to get topic stats
get_stats() {
    echo "📊 Getting topic stats:"
    $CLI --server $SERVER_URL topics stats --name test-topic
}

# Main test sequence
main() {
    check_server || exit 1
    test_cli || exit 1
    create_topic
    publish_message || exit 1
    list_topics
    echo ""
    get_stats
    
    echo ""
    echo -e "${GREEN}🎉 System test completed successfully!${NC}"
    echo ""
    echo "To test real-time subscription, run in another terminal:"
    echo "  $CLI subscribe --topic test-topic"
    echo ""
    echo "Then publish more messages:"
    echo "  $CLI publish --topic test-topic --message 'Live message!'"
}

main "$@"