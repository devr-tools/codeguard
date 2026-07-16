package checks_test

import (
	"path/filepath"
	"testing"
)

func TestCPPGoogleTestQualityRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tests", "widget_test.cpp"), `#include <gtest/gtest.h>

TEST(WidgetTest, NoAssertion) {
    auto value = compute();
    (void)value;
}

TEST_F(WidgetTest, AlwaysTrue) {
    EXPECT_TRUE(true);
}

TEST_P(WidgetTest, ConditionalAssertion) {
    auto value = compute();
    if (value > 0) {
        ASSERT_EQ(value, 5);
    }
}

TYPED_TEST(WidgetTypedTest, ProperAssertion) {
    EXPECT_EQ(compute(), 5);
}

TEST(
    WidgetTest,
    MultilineDeclaration
) {
    EXPECT_NE(compute(), 0);
}

TEST(WidgetTest, ExplicitFailureBranch) {
    if (compute() != 5) {
        ADD_FAILURE() << "unexpected value";
    }
}
`)

	report := runScan(t, testQualityConfig(t, dir, "c++"))

	assertRuleCount(t, report, "ci.test-without-assertion", 1)
	assertRuleCount(t, report, "ci.always-true-test-assertion", 1)
	assertRuleCount(t, report, "ci.conditional-assertion", 1)
}

func TestCPPCatchDoctestAndBoostAssertions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tests", "frameworks_test.cpp"), `#include <boost/test/unit_test.hpp>
#include <catch2/catch_test_macros.hpp>

TEST_CASE("Catch2 assertion", "[widget]") {
    CHECK(compute() == 5);
}

TEST_CASE_FIXTURE(WidgetFixture, "doctest fixture assertion") {
    REQUIRE_FALSE(compute() == 0);
}

TEMPLATE_TEST_CASE("Catch2 template assertion", "[widget]", int, long) {
    CHECK_THAT(compute(), MatchesWidget());
}

BOOST_AUTO_TEST_CASE(boost_assertion) {
    BOOST_CHECK_EQUAL(compute(), 5);
}

BOOST_FIXTURE_TEST_CASE(boost_fixture_assertion, WidgetFixture) {
    BOOST_REQUIRE(compute() == 5);
}
`)

	report := runScan(t, testQualityConfig(t, dir, "cpp"))

	assertRuleCount(t, report, "ci.test-without-assertion", 0)
	assertRuleCount(t, report, "ci.always-true-test-assertion", 0)
	assertRuleCount(t, report, "ci.conditional-assertion", 0)
}
